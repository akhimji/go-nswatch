package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
)

func parseK8sYaml(fileR []byte) ([]string, []string) {
	var yamlDeployments []string
	var svcDeployments []string
	acceptedK8sTypes := regexp.MustCompile(`(Role|ClusterRole|RoleBinding|ClusterRoleBinding|ServiceAccount|Deployment|Service)`)
	fileAsString := string(fileR[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	//retVal := make([]runtime.Object, 0, len(sepYamlfiles))
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(f), nil, nil)
		if err != nil {
			log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}

		if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
			log.Printf("The custom-roles configMap contained K8s object types which are not supported! Skipping object with type: %s", groupVersionKind.Kind)
		} else {
			if groupVersionKind.Kind == "Deployment" {
				//fmt.Println("This is type Deployment")
				//b := []byte(f)
				//createDeploymentFromYaml(clientset, b, "hipster-cli")
				//fmt.Println(f)
				deployment := obj.(*appsv1.Deployment)
				//fmt.Println("Name:", deployment.GetName())
				yamlDeployments = append(yamlDeployments, deployment.GetName())

			}
			if groupVersionKind.Kind == "Service" {
				services := obj.(*v1.Service)
				//fmt.Println("Name:", deployment.GetName())
				svcDeployments = append(svcDeployments, services.GetName())
			}
		}

	}
	return yamlDeployments, svcDeployments
}

func repairdeployment(fileR []byte, repairdep string, namespace string, clientset *kubernetes.Clientset) {
	fileAsString := string(fileR[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	//retVal := make([]runtime.Object, 0, len(sepYamlfiles))
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(f), nil, nil)
		if err != nil {
			log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}

		if groupVersionKind.Kind == "Deployment" {
			deployment := obj.(*appsv1.Deployment)
			if deployment.GetName() == repairdep {
				log.Println("Repairing Missing Deployment:", deployment.GetName())
				b := []byte(f)
				createDeploymentFromYaml(clientset, b, namespace)
			}

		}

	}
}

func repairservice(fileR []byte, repairsv string, namespace string, clientset *kubernetes.Clientset) {
	fileAsString := string(fileR[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	//retVal := make([]runtime.Object, 0, len(sepYamlfiles))
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(f), nil, nil)
		if err != nil {
			log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}

		if groupVersionKind.Kind == "Service" {
			service := obj.(*v1.Service)
			if service.GetName() == repairsv {
				log.Println("Repairing Missing Service:", service.GetName())
				b := []byte(f)
				createServiceFromYaml(clientset, b, namespace)
			}
		}

	}
}

func buildClient() (*kubernetes.Clientset, error) {
	var kubeconfig string
	var cfgFile string
	if cfgFile != "" {
		kubeconfig = cfgFile
		log.Println(" ✓ Using kubeconfig file via flag: ", kubeconfig)
	} else {
		kubeconfig = os.Getenv("kubeconfig")
		if kubeconfig != "" {
			log.Println(" ✓ Using kubeconfig via OS ENV")
		} else {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
			if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
				log.Println(" X kubeconfig Not Found, use --kubeconfig")
				os.Exit(1)
			} else {
				log.Println(" ✓ Using kubeconfig file via homedir: ", kubeconfig)
			}

		}

	}

	// Bootstrap k8s configuration
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	return clientset, err
}

func createServiceFromYaml(clientset *kubernetes.Clientset, podAsYaml []byte, ns string) {
	log.Println("Attempting Service Deployment..")
	var p v1.Service
	err := yaml.Unmarshal(podAsYaml, &p)
	if err != nil {
		log.Println("Error Unmarshalling: ", err)
	}
	service, err := clientset.CoreV1().Services(ns).Create(&p)
	if err != nil {
		log.Println("Error creating service: ", err)
	}
	fmt.Printf("Created Service %q.\n", service.GetObjectMeta().GetName())
}

func createDeploymentFromYaml(clientset *kubernetes.Clientset, podAsYaml []byte, ns string) error {
	log.Println("Attempting Deployment..")
	var deployment appsv1.Deployment
	err := yaml.Unmarshal(podAsYaml, &deployment)
	if err != nil {
		log.Println("Error Unmarshaling:", err)
	}

	deploymentsClient := clientset.AppsV1().Deployments(ns)
	result, err := deploymentsClient.Create(&deployment)
	//pod, poderr := clientset.CoreV1().Pods(ns).Create(&deployment)
	if err != nil {
		log.Println("Error Creating Deployment:")
		log.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	return nil
}

func getDeployment(namespace string, name string, client *kubernetes.Clientset) *appsv1.Deployment {
	d, err := client.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	return d
}

func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [inputfile]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}

func getNamespaces(clientset *kubernetes.Clientset, ns string) {
	var ok string
	namespace, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		log.Fatalln("Failed to get namespace:", err)
		os.Exit(1)
	} else {
		for _, name := range namespace.Items {
			if name.GetName() == ns {
				ok = "found"
				break
			}
			ok = "not found"
		}
	}
	if ok == "not found" {
		log.Fatalln("Namespace Not Found..")
		os.Exit(1)
	}

}

func main() {
	var namespace string

	clientset, err := buildClient()
	if len(os.Args) != 2 {
		log.Println("Usage:", os.Args[0], "namespace")
		os.Exit(1)
	} else if len(os.Args) == 2 {
		namespace = os.Args[1]
		log.Println("Watching Namespace: ", namespace)
	}
	getNamespaces(clientset, namespace)

	var yamlDeployments []string
	var yamlServices []string
	fileURL := "https://raw.githubusercontent.com/GoogleCloudPlatform/microservices-demo/master/release/kubernetes-manifests.yaml"
	if err := downloadFile("/tmp/hipster.yaml", fileURL); err != nil {
		panic(err)
	}
	data, err := ioutil.ReadFile("/tmp/hipster.yaml")
	if err != nil {
		log.Println("File reading error", err)
		os.Exit(1)
	}

	// strip out comments from yaml file //
	re := regexp.MustCompile("(?m)[\r\n]+^.*#.*$")
	res := re.ReplaceAllString(string(data), "")
	data = []byte(res)
	// --- //

	yamlDeployments, yamlServices = parseK8sYaml(data)

	log.Println("Starting Watch Loop...")
	for {
		var currentDeployments []string
		var currentServices []string
		deploymentsClient := clientset.AppsV1().Deployments(namespace)
		deployments, _ := deploymentsClient.List(metav1.ListOptions{})
		services, _ := clientset.CoreV1().Services(namespace).List(metav1.ListOptions{})

		for _, d := range deployments.Items {
			a := getDeployment(namespace, d.Name, clientset)
			currentDeployments = append(currentDeployments, a.GetName())
			//fmt.Println("Current Deployment: ", d.GetName())
			//fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
		}

		for _, services := range services.Items {
			//fmt.Println("Current Service: ", services.GetName())
			currentServices = append(currentServices, services.GetName())
		}

		//debugging//
		//fmt.Println("yaml dep parsed:", yamlDeployments)
		//fmt.Println("cuurent dep:", currentDeployments)
		//fmt.Println("yaml svc parsed:", yamlServices)

		repairDep := difference(yamlDeployments, currentDeployments)
		repairSvc := difference(yamlServices, currentServices)

		//fmt.Println("Dep Delta:", repairDep)
		//fmt.Println("Svc Delta:", repairSvc)

		for _, name := range repairDep {
			repairdeployment(data, name, namespace, clientset)
			//fmt.Println(name)
		}
		for _, name := range repairSvc {
			repairservice(data, name, namespace, clientset)
			//fmt.Println(name)
		}
		time.Sleep(10 * time.Second)

	}
}
