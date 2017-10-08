// Copyright (c) Alex Ellis, Eric Stoekl 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.
package commands

import (
	"errors"

	"fmt"

	"os"

	"regexp"

	"github.com/spf13/cobra"
)

// Args and Flags that are to be added to commands

const (
	repositoryRegexpMockedServer = `^http://127.0.0.1:\d+/([a-z0-9-]+)/([a-z0-9-]+)$`
	repositoryRegexpGithub       = `^https://github.com/([a-z0-9-]+)/([a-z0-9-]+)$`
)

var (
	repository string
	overwrite  bool
)

func init() {
	addTemplateCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing templates?")

	faasCmd.AddCommand(addTemplateCmd)
}

// addTemplateCmd allows the user to fetch a template from a repository
var addTemplateCmd = &cobra.Command{
	Use: "add-template <repository URL>",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("A repository URL must be specified")
		} else {
			var validURL = regexp.MustCompile(repositoryRegexpGithub + "|" + repositoryRegexpMockedServer)

			if !validURL.MatchString(args[0]) {
				return errors.New("The repository URL must be in the format https://github.com/<owner>/<repository>")
			}
		}
		return nil
	},
	Short: "Downloads templates from the specified github repo",
	Long: `Downloads the compressed github repo specified by [URL], and extracts the 'template'
	directory from the root of the repo, if it exists.`,
	Example: "faas-cli add-template https://github.com/alexellis/faas-cli",
	Run:     runAddTemplate,
}

func runAddTemplate(cmd *cobra.Command, args []string) {
	repository = args[0]

	fmt.Println("Get " + repository)
	if err := fetchTemplates(repository, overwrite); err != nil {
		fmt.Println(err)

		os.Exit(1)
	}
}
