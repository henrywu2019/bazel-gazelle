/* Copyright 2019 The Bazel Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Command generate_repo_config takes in a build config file such as
// WORKSPACE and generates a stripped version of the file. The generated
// config file should contain only the information relevant to gazelle for
// dependency resolution, so go_repository rules with importpath
// and name defined, plus any directives.
//
// This command is used by the go_repository_config rule to generate a repo
// config file used by all go_repository rules.
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"sort"

	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

var (
	configSource = flag.String("config_source", "", "a file that is read to learn about external repositories")
	configDest   = flag.String("config_dest", "", "destination file for the generated repo config")
)

type byName []repo.Repo

func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func main() {
	log.SetFlags(0)
	log.SetPrefix("generate_repo_config: ")

	flag.Parse()
	if *configDest == "" {
		log.Fatal("-config_dest must be set")
	}
	if *configSource == "" {
		log.Fatal("-config_source must be set")
	}
	if flag.NArg() != 0 {
		log.Fatal("generate_repo_config does not accept positional arguments")
	}
	if err := generateRepoConfig(*configDest, *configSource); err != nil {
		log.Fatal(err)
	}
}

func generateRepoConfig(configDest, configSource string) error {
	var buf bytes.Buffer
	buf.WriteString("# Code generated by generate_repo_config.go; DO NOT EDIT.\n")

	sourceFile, err := rule.LoadWorkspaceFile(configSource, "")
	if err != nil {
		return err
	}
	repos, reposByFile, err := repo.ListRepositories(sourceFile)
	if err != nil {
		return err
	}
	sort.Stable(byName(repos))

	sortedRepoFiles := make([]*rule.File, 0, len(reposByFile))
	for r := range reposByFile {
		sortedRepoFiles = append(sortedRepoFiles, r)
	}
	sort.SliceStable(sortedRepoFiles, func(i, j int) bool {
		return sortedRepoFiles[i].Path < sortedRepoFiles[j].Path
	})
	for _, r := range sortedRepoFiles {
		for _, d := range r.Directives {
			// skip repository_macro directives, because for the repo config we flatten
			// macros into one file
			if d.Key != "repository_macro" {
				buf.WriteString("# gazelle:" + d.Key + " " + d.Value + "\n")
			}
		}
	}

	destFile := rule.EmptyFile(configDest, "")
	for _, r := range repos {
		if r.Name != "" && r.GoPrefix != "" {
			rule := rule.NewRule("go_repository", r.Name)
			rule.SetAttr("importpath", r.GoPrefix)
			rule.Insert(destFile)
		}
	}

	buf.WriteString("\n")
	buf.Write(destFile.Format())
	if err := ioutil.WriteFile(configDest, buf.Bytes(), 0666); err != nil {
		return err
	}

	return nil
}
