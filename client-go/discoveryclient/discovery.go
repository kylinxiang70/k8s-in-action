/**
 * @author xiangqilin
 * @date 2020/12/4
**/
package main

import (
	"fmt"
	"github.com/prometheus/common/log"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path"
)

const defaultKubeConfigPath = ".kube/config"

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	configPath := path.Join(homeDir, defaultKubeConfigPath)
	log.Info(fmt.Sprintf("kubeconfig path: %s", configPath))

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		panic(err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		panic(err)
	}

	apiGroup, apiResourceList, err := discoveryClient.ServerGroupsAndResources()

	fmt.Println("====================================================")

	// 打印Group的Version
	for i := 0; i < len(apiGroup); i++ {
		name := apiGroup[i].Name
		versions := apiGroup[i].Versions
		for j := 0; j < len(versions); j++ {
			fmt.Println(fmt.Sprintf("Group: %s, Version: %s", name, versions[j].Version))
		}
	}

	fmt.Println("====================================================")

	// 打印GroupVersion的Resources
	for i := 0; i < len(apiResourceList); i++ {
		gv := apiResourceList[i].GroupVersion
		apiResources := apiResourceList[i].APIResources
		for j := 0; j < len(apiResources); j++ {
			name := apiResources[j].Name
			fmt.Println(fmt.Sprintf("GourpVersion: %s, APIResource: %s", gv, name))
		}
	}

}
