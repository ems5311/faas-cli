// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/faas-cli/stack"
	"github.com/spf13/cobra"
)

// Flags that are to be added to commands.

var (
	envvarOpts  []string
	replace     bool
	update      bool
	constraints []string
	secrets     []string
	labelOpts   []string
)

func init() {
	// Setup flags that are used by multiple commands (variables defined in faas.go)
	deployCmd.Flags().StringVar(&fprocess, "fprocess", "", "Fprocess to be run by the watchdog")
	deployCmd.Flags().StringVar(&gateway, "gateway", defaultGateway, "Gateway URI")
	deployCmd.Flags().StringVar(&handler, "handler", "", "Directory with handler for function, e.g. handler.js")
	deployCmd.Flags().StringVar(&image, "image", "", "Docker image name to build")
	deployCmd.Flags().StringVar(&language, "lang", "", "Programming language template")
	deployCmd.Flags().StringVar(&functionName, "name", "", "Name of the deployed function")
	deployCmd.Flags().StringVar(&network, "network", defaultNetwork, "Name of the network")

	// Setup flags that are used only by this command (variables defined above)
	deployCmd.Flags().StringArrayVarP(&envvarOpts, "env", "e", []string{}, "Set one or more environment variables (ENVVAR=VALUE)")

	deployCmd.Flags().StringArrayVarP(&labelOpts, "label", "l", []string{}, "Set one or more label (LABEL=VALUE)")

	deployCmd.Flags().BoolVar(&replace, "replace", true, "Replace any existing function")
	deployCmd.Flags().BoolVar(&update, "update", false, "Update existing functions")

	deployCmd.Flags().StringArrayVar(&constraints, "constraint", []string{}, "Apply a constraint to the function")
	deployCmd.Flags().StringArrayVar(&secrets, "secret", []string{}, "Give the function access to a secure secret")

	// Set bash-completion.
	_ = deployCmd.Flags().SetAnnotation("handler", cobra.BashCompSubdirsInDir, []string{})

	faasCmd.AddCommand(deployCmd)
}

// deployCmd handles deploying OpenFaaS function containers
var deployCmd = &cobra.Command{
	Use: `deploy -f YAML_FILE [--replace=false]
  faas-cli deploy --image IMAGE_NAME
                  --name FUNCTION_NAME
                  [--lang <ruby|python|node|csharp>]
                  [--gateway GATEWAY_URL]
                  [--network NETWORK_NAME]
                  [--handler HANDLER_DIR]
                  [--fprocess PROCESS]
                  [--env ENVVAR=VALUE ...]
                  [--label LABEL=VALUE ...]
				  [--replace=false]
				  [--update=false]
                  [--constraint PLACEMENT_CONSTRAINT ...]
                  [--regex "REGEX"]
                  [--filter "WILDCARD"]
				  [--secret "SECRET_NAME"]`,

	Short: "Deploy OpenFaaS functions",
	Long: `Deploys OpenFaaS function containers either via the supplied YAML config using
the "--yaml" flag (which may contain multiple function definitions), or directly
via flags. Note: --replace and --update are mutually exclusive.`,
	Example: `  faas-cli deploy -f https://domain/path/myfunctions.yml
  faas-cli deploy -f ./samples.yml
  faas-cli deploy -f ./samples.yml --label canary=true
  faas-cli deploy -f ./samples.yml --filter "*gif*" --secret dockerhuborg
  faas-cli deploy -f ./samples.yml --regex "fn[0-9]_.*"
  faas-cli deploy -f ./samples.yml --replace=false
  faas-cli deploy -f ./samples.yml --update=true
  faas-cli deploy --image=alexellis/faas-url-ping --name=url-ping
  faas-cli deploy --image=my_image --name=my_fn --handler=/path/to/fn/
                  --gateway=http://remote-site.com:8080 --lang=python
                  --env=MYVAR=myval`,
	Run: runDeploy,
}

func runDeploy(cmd *cobra.Command, args []string) {

	if update && replace {
		fmt.Println(`Cannot specify --update and --replace at the same time.
  --replace    removes an existing deployment before re-creating it
  --update     provides a rolling update to a new function image or configuration`)
		return
	}

	var services stack.Services
	if len(yamlFile) > 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter)
		if err != nil {
			log.Fatalln(err.Error())
			return
		}

		parsedServices.Provider.GatewayURL = getGatewayURL(gateway, defaultGateway, parsedServices.Provider.GatewayURL)

		// Override network if passed
		if len(network) > 0 && network != defaultNetwork {
			parsedServices.Provider.Network = network
		}

		if parsedServices != nil {
			services = *parsedServices
		}
	}

	if len(services.Functions) > 0 {
		if len(services.Provider.Network) == 0 {
			services.Provider.Network = defaultNetwork
		}

		for k, function := range services.Functions {
			function.Name = k
			if update {
				fmt.Printf("Updating: %s.\n", function.Name)
			} else {
				fmt.Printf("Deploying: %s.\n", function.Name)
			}
			if function.Constraints != nil {
				constraints = *function.Constraints
			}

			fileEnvironment, err := readFiles(function.EnvironmentFile)
			if err != nil {
				log.Fatalln(err)
			}

			labelMap := map[string]string{}
			if function.Labels != nil {
				labelMap = *function.Labels
			}

			labelArgumentMap, labelErr := parseMap(labelOpts, "label")
			if labelErr != nil {
				fmt.Printf("Error parsing labels: %v\n", labelErr)
				os.Exit(1)
			}

			allLabels := mergeMap(labelMap, labelArgumentMap)

			allEnvironment, envErr := compileEnvironment(envvarOpts, function.Environment, fileEnvironment)
			if envErr != nil {
				log.Fatalln(envErr)
			}

			// Get FProcess to use from the ./template/template.yml, if a template is being used
			if function.Language != "" && function.Language != "Dockerfile" && function.Language != "dockerfile" {
				pathToTemplateYAML := "./template/" + function.Language + "/template.yml"
				if _, err := os.Stat(pathToTemplateYAML); os.IsNotExist(err) {
					log.Fatalln(err.Error())
					return
				}

				var langTemplate stack.LanguageTemplate
				parsedLangTemplate, err := stack.ParseYAMLForLanguageTemplate(pathToTemplateYAML)

				if err != nil {
					log.Fatalln(err.Error())
					return
				}

				if parsedLangTemplate != nil {
					langTemplate = *parsedLangTemplate
					function.FProcess = langTemplate.FProcess
				}
			}

			proxy.DeployFunction(function.FProcess, services.Provider.GatewayURL, function.Name, function.Image, function.Language, replace, allEnvironment, services.Provider.Network, constraints, update, secrets, allLabels)
		}
	} else {
		if len(image) == 0 {
			fmt.Println("Please provide a --image to be deployed.")
			return
		}
		if len(functionName) == 0 {
			fmt.Println("Please provide a --name for your function as it will be deployed on FaaS")
			return
		}

		envvars, err := parseMap(envvarOpts, "env")
		if err != nil {
			fmt.Printf("Error parsing envvars: %v\n", err)
			os.Exit(1)
		}

		labelMap, labelErr := parseMap(labelOpts, "label")
		if labelErr != nil {
			fmt.Printf("Error parsing labels: %v\n", labelErr)
			os.Exit(1)
		}

		proxy.DeployFunction(fprocess, gateway, functionName, image, language, replace, envvars, network, constraints, update, secrets, labelMap)
	}
}

func buildLabelMap(labelOpts []string) map[string]string {
	labelMap := map[string]string{}
	for _, opt := range labelOpts {
		if !strings.Contains(opt, "=") {
			fmt.Println("Error - label option does not contain a value")
		} else {
			index := strings.Index(opt, "=")

			labelMap[opt[0:index]] = opt[index+1:]
		}
	}
	return labelMap
}

func readFiles(files []string) (map[string]string, error) {
	envs := make(map[string]string)

	for _, file := range files {
		bytesOut, readErr := ioutil.ReadFile(file)
		if readErr != nil {
			return nil, readErr
		}

		envFile := stack.EnvironmentFile{}
		unmarshalErr := yaml.Unmarshal(bytesOut, &envFile)
		if unmarshalErr != nil {
			return nil, unmarshalErr
		}
		for k, v := range envFile.Environment {
			envs[k] = v
		}
	}
	return envs, nil
}

func parseMap(envvars []string, keyName string) (map[string]string, error) {
	result := make(map[string]string)
	for _, envvar := range envvars {
		s := strings.SplitN(strings.TrimSpace(envvar), "=", 2)
		envvarName := s[0]
		envvarValue := s[1]

		if !(len(envvarName) > 0) {
			return nil, fmt.Errorf("Empty %s name: [%s]", keyName, envvar)
		}
		if !(len(envvarValue) > 0) {
			return nil, fmt.Errorf("Empty %s value: [%s]", keyName, envvar)
		}

		result[envvarName] = envvarValue
	}
	return result, nil
}

func mergeMap(i map[string]string, j map[string]string) map[string]string {
	merged := make(map[string]string)

	for k, v := range i {
		merged[k] = v
	}
	for k, v := range j {
		merged[k] = v
	}
	return merged
}

func getGatewayURL(argumentURL string, defaultURL string, yamlURL string) string {
	var gatewayURL string

	if len(argumentURL) > 0 && argumentURL != defaultURL {
		gatewayURL = argumentURL
	} else if len(yamlURL) > 0 {
		gatewayURL = yamlURL
	} else {
		gatewayURL = defaultURL
	}

	return gatewayURL
}

func compileEnvironment(envvarOpts []string, yamlEnvironment map[string]string, fileEnvironment map[string]string) (map[string]string, error) {
	envvarArguments, err := parseMap(envvarOpts, "env")
	if err != nil {
		return nil, fmt.Errorf("error parsing envvars: %v", err)
	}

	functionAndStack := mergeMap(yamlEnvironment, fileEnvironment)
	return mergeMap(functionAndStack, envvarArguments), nil
}
