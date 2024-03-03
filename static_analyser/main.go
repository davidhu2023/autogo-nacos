package main

import (
	"encoding/json"
	"fmt"

	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// set the output directory for manifests
var outputPrefix = "output/game_microservices/"

var nacos_functions = []string{"RegisterInstance", "GetService", "SelectAllInstances", "SelectOneHealthyInstance", "SelectInstances", "Subscribe"}

// Set the root directory you want to search
var root = "../game_microservices/"

func main() {
	var validYamlFiles []string
	parsedYamls := make(map[string]*Yaml2Go) // Initialize the map

	// map to store the TCPManifest for each application
	var application_to_manifest = make(map[string]TCPManifest)

	// store the files with nacos functions for each application
	var nacos_files = make(map[string][]string)

	// store the folders with containing the code for each application
	var application_folders = make(map[string]string)

	//store the information for each service registrated to nacos for each application
	// maps application to service information
	var service_directory = make(map[string]ServiceInfo)

	// store the service discovery calls to nacos for each application
	var call_map = make(map[string][]TCPRequest)

	// Walk thru all files in root dir and find yaml files with ApiVersion and Kind fields
	// Parse YAML files and store in map with service name
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
			conf, serviceName, err := ParseYaml(path) // Correctly handle returned values
			if err != nil {
				fmt.Printf("Error parsing YAML file %s: %v\n", path, err)
				return nil // Continue processing other files even if this one fails
			}

			// Assuming you want to track YAML files that successfully parsed and contained the app label
			if serviceName != "" {
				parsedYamls[serviceName] = conf
				application_folders[serviceName] = filepath.Dir(path)
				validYamlFiles = append(validYamlFiles, path)

			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the file tree: %v\n", err)
		return
	}

	fmt.Println("Valid .yaml files with required fields:")
	for _, file := range validYamlFiles {
		fmt.Println(file)
	}

	// Create TCPManifest for each service
	for application, value := range parsedYamls {
		log.Printf("Service: %s, Version: %s", application, value.Metadata.Labels.Version)
		version := value.Metadata.Labels.Version
		application_to_manifest[application] = TCPManifest{Version: version, Service: application}
	}

	// Output the TCPManifest for each service in JSON format
	for _, manifest := range application_to_manifest {
		PrintJson(manifest)
	}
	var register_wrapper_map = make(map[string]RegisterInfo)
	var select_wrapper_map = make(map[string]SelectInfo)

	// Only looks at the directories corresponding to each service.
	// Loop for registration  calls
	for application, dir := range application_folders {
		// Find all the go files
		goFiles := FindGoFiles(dir)
		// Find all the go files with sdk function calls
		nacos_files = FindGoFilesWithFunctions(dir, nacos_functions)

		// Print the occurrences
		// for funcName, nacos_files := range nacos_files {
		// 	fmt.Printf("Function %s is called in: %v by application %s", funcName, nacos_files, application)
		// }

		for _, files := range nacos_files {

			for _, file := range files {
				f := parseFile(file)

				if err != nil {
					log.Fatal(err)
				}

				instances := RegisterWrappers(f)
				log.Printf("%v", instances)
				for _, instance := range instances {
					register_wrapper_map[application] = instance
				}
			}

			for _, file := range goFiles {
				f := parseFile(file)
				if err != nil {
					log.Fatal(err)
				}
				for key, value := range register_wrapper_map {
					if key == application {
						names, infos := RegisterCalls(f, value, application)
						for i, name := range names {
							service_directory[name] = infos[i]
						}
					}
				}
			}

			log.Printf("\nService Map: %v", service_directory)

		}

	}

	// Loop for service discovery calls
	for application, dir := range application_folders {
		goFiles := FindGoFiles(dir)
		// Find all the go files with sdk function calls
		nacos_files = FindGoFilesWithFunctions(dir, nacos_functions)

		// Print the occurrences
		// for funcName, nacos_files := range nacos_files {
		// 	fmt.Printf("Function %s is called in: %v by service %s", funcName, nacos_files, service)
		// }

		for _, files := range nacos_files {

			for _, file := range files {
				f := parseFile(file)

				if err != nil {
					log.Fatal(err)
				}
				instances := DiscoveryWrappers(f)
				for _, instance := range instances {
					select_wrapper_map[application] = instance

				}
			}
		}

		for _, file := range goFiles {

			f := parseFile(file)
			if err != nil {
				log.Fatal(err)
			}
			for key, value := range select_wrapper_map {
				if key == application {
					names := DiscoveryCalls(f, value, application)
					for _, name := range names {
						req := TCPRequest{Type: "tcp", URL: service_directory[name].IP, Name: service_directory[name].application, Port: service_directory[name].Port}
						call_map[application] = append(call_map[application], req)
					}

				}

			}
		}

		log.Printf("%v", call_map)
	}

	// Outputs the TCPManifest file for each service in JSON format
	log.Printf("%v", application_to_manifest)
	for application := range application_folders {
		log.Printf("Service: %s", application)
		temp := application_to_manifest[application]
		temp.Requests = call_map[application]
		application_to_manifest[application] = temp

		TCPOutput(application_to_manifest[application])

	}

}

func TCPOutput(mani TCPManifest) {
	b, err := json.MarshalIndent(mani, "", " ")
	if err != nil {
		fmt.Println("error:", err)
	}
	//生成json文件
	err = ioutil.WriteFile(outputPrefix+mani.Service+".json", b, 0777)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var data interface{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		fmt.Println("error:", err)
	}

}