/*
Copyright 2020 The Knative Authors.

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

package generators

import (
	"io"

	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog"
)

// reconcilerFuncGenerator produces a file ...
type reconcilerFuncGenerator struct {
	generator.DefaultGen
	outputPackage string
	imports       namer.ImportTracker
	filtered      bool

	clientPkg           string
	informerPackagePath string
}

var _ generator.Generator = (*reconcilerFuncGenerator)(nil)

func (g *reconcilerFuncGenerator) Filter(c *generator.Context, t *types.Type) bool {
	// We generate a single client, so return true once.
	if !g.filtered {
		g.filtered = true
		return true
	}
	return false
}

func (g *reconcilerFuncGenerator) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *reconcilerFuncGenerator) Imports(c *generator.Context) (imports []string) {
	imports = append(imports, g.imports.ImportLines()...)
	return
}

func (g *reconcilerFuncGenerator) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	klog.V(5).Infof("processing type %v", t)

	m := map[string]interface{}{
		"type": t,
		"clientGet": c.Universe.Function(types.Name{
			Package: g.clientPkg,
			Name:    "Get",
		}),
		"informerGet": c.Universe.Function(types.Name{
			Package: g.informerPackagePath,
			Name:    "Get",
		}),
		"getControllerOf": c.Universe.Function(types.Name{
			Package: "k8s.io/apimachinery/pkg/apis/meta/v1",
			Name:    "GetControllerOf",
		}),
	}
	sw.Do(reconcilerFuncNewImpl, m)

	return sw.Error()
}

var reconcilerFuncNewImpl = `
func Reconcile(ctx context.Context, desired *{{.type|raw}}) (*{{.type|raw}}, error) {
	client := {{.clientGet|raw}}(ctx)
	lister := {{.informerGet|raw}}(ctx).Lister()

	actual, err := lister.{{.type|apiGroup}}(desired.Namespace).Get(desired.Name)
	if errors.IsNotFound(err) {
		created, err := client.{{.type|versionedClientset}}().{{.type|apiGroup}}(desired.Namespace).Create(desired)
		if err != nil {
			return nil, fmt.Errorf("error creating {{.type|public}}: %w", err)
		}
		return created, nil
	} else if err != nil {
		return nil, fmt.Errorf("error fetching {{.type|public}}: %w", err)
	} else if *{{.getControllerOf|raw}}(actual) != *{{.getControllerOf|raw}}(desired) {
		return nil, fmt.Errorf("not owned error (should be static to be checkable)")
	} else {
		if !equality.Semantic.DeepEqual(desired.Spec, actual.Spec) {
			want := actual.DeepCopy()
			want.Spec = desired.Spec
			updated, err := client.{{.type|versionedClientset}}().{{.type|apiGroup}}(want.Namespace).Update(want)
			if err != nil {
				return nil, fmt.Errorf("error updating {{.type|public}}: %w", err)
			}
			return updated, nil
		}
		return actual, nil
	}
}
`
