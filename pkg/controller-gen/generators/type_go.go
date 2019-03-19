package generators

import (
	"fmt"
	"io"
	"strings"

	args2 "github.com/rancher/wrangler/pkg/controller-gen/args"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/code-generator/cmd/client-gen/generators/util"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
)

func TypeGo(gv schema.GroupVersion, name *types.Name, args *args.GeneratorArgs, customArgs *args2.CustomArgs) generator.Generator {
	return &typeGo{
		name:       name,
		gv:         gv,
		args:       args,
		customArgs: customArgs,
		DefaultGen: generator.DefaultGen{
			OptionalName: strings.ToLower(name.Name),
		},
	}
}

type typeGo struct {
	generator.DefaultGen

	name       *types.Name
	gv         schema.GroupVersion
	args       *args.GeneratorArgs
	customArgs *args2.CustomArgs
}

func (f *typeGo) Imports(*generator.Context) []string {
	group := f.customArgs.Options.Groups[f.gv.Group]

	packages := []string{
		"metav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\"",
		"k8s.io/apimachinery/pkg/labels",
		"k8s.io/apimachinery/pkg/runtime",
		"k8s.io/apimachinery/pkg/runtime/schema",
		"k8s.io/apimachinery/pkg/types",
		"utilruntime \"k8s.io/apimachinery/pkg/util/runtime\"",
		"k8s.io/apimachinery/pkg/watch",
		"k8s.io/client-go/tools/cache",
		f.name.Package,
		GenericPackage,
		fmt.Sprintf("clientset \"%s/typed/%s/%s\"", group.ClientSetPackage, f.gv.Group, f.gv.Version),
		fmt.Sprintf("informers \"%s/%s/%s\"", group.InformersPackage, f.gv.Group, f.gv.Version),
		fmt.Sprintf("listers \"%s/%s/%s\"", group.ListersPackage, f.gv.Group, f.gv.Version),
	}

	return packages
}

func (f *typeGo) Init(c *generator.Context, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")

	if err := f.DefaultGen.Init(c, w); err != nil {
		return err
	}

	t := c.Universe.Type(*f.name)
	m := map[string]interface{}{
		"type":       f.name.Name,
		"lowerName":  namer.IL(f.name.Name),
		"plural":     plural.Name(t),
		"version":    f.gv.Version,
		"namespaced": !util.MustParseClientGenTags(t.SecondClosestCommentLines).NonNamespaced,
	}

	sw.Do(string(typeBody), m)
	return sw.Error()
}

var typeBody = `
type {{.type}}Handler func(string, *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)

type {{.type}}Controller interface {
	Create(*{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)
	Update(*{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)
	UpdateStatus(*{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error)
	Delete({{ if .namespaced}}namespace, {{end}}name string, options *metav1.DeleteOptions) error
	DeleteCollection({{ if .namespaced}}namespace string, {{end}}options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get({{ if .namespaced}}namespace, {{end}}name string, options metav1.GetOptions) (*{{.version}}.{{.type}}, error)
	List({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (*{{.version}}.{{.type}}List, error)
	Watch({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (watch.Interface, error)
	Patch({{ if .namespaced}}namespace, {{end}}name string, pt types.PatchType, data []byte, subresources ...string) (result *{{.version}}.{{.type}}, err error)

	Cache() {{.type}}ControllerCache

	OnChange(ctx context.Context, name string, sync {{.type}}Handler)
	OnRemove(ctx context.Context, name string, sync {{.type}}Handler)
	Enqueue({{ if .namespaced}}namespace, {{end}}name string)
}

type {{.type}}ControllerCache interface {
	Get({{ if .namespaced}}namespace, {{end}}name string) (*{{.version}}.{{.type}}, error)
	List({{ if .namespaced}}namespace string, {{end}}selector labels.Selector) ([]*{{.version}}.{{.type}}, error)

	AddIndexer(indexName string, indexer {{.type}}Indexer)
	GetByIndex(indexName, key string) ([]*{{.version}}.{{.type}}, error)
}

type {{.type}}Indexer func(obj *{{.version}}.{{.type}}) ([]string, error)

type {{.lowerName}}Controller struct {
	controllerManager *generic.ControllerManager
	clientGetter      clientset.{{.plural}}Getter
	informer          informers.{{.type}}Informer
	gvk               schema.GroupVersionKind
}

func New{{.type}}Controller(gvk schema.GroupVersionKind, controllerManager *generic.ControllerManager, clientGetter clientset.{{.plural}}Getter, informer informers.{{.type}}Informer) {{.type}}Controller {
	return &{{.lowerName}}Controller{
		controllerManager: controllerManager,
		clientGetter:      clientGetter,
		informer:          informer,
		gvk:               gvk,
	}
}

func from{{.type}}HandlerToHandler(sync {{.type}}Handler) generic.Handler {
	return func(key string, obj runtime.Object) (runtime.Object, error) {
		obj, err := sync(key, obj.(*{{.version}}.{{.type}}))
		if obj == nil {
			return nil, err
		}
		return obj, err
	}
}

func (c *{{.lowerName}}Controller) updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*{{.version}}.{{.type}}))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func (c *{{.lowerName}}Controller) addHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controllerManager.AddHandler(ctx, c.gvk, c.informer.Informer(), name, handler)
}

func (c *{{.lowerName}}Controller) OnChange(ctx context.Context, name string, sync {{.type}}Handler) {
	c.addHandler(ctx, name, from{{.type}}HandlerToHandler(sync))
}

func (c *{{.lowerName}}Controller) OnRemove(ctx context.Context, name string, sync {{.type}}Handler) {
	removeHandler := generic.NewRemoveHandler(name, c.updater(), from{{.type}}HandlerToHandler(sync))
	c.addHandler(ctx, name, removeHandler)
}

func (c *{{.lowerName}}Controller) Enqueue({{ if .namespaced}}namespace, {{end}}name string) {
	c.controllerManager.Enqueue(c.gvk, {{ if .namespaced }}namespace, {{else}}"", {{end}}name)
}

func (c *{{.lowerName}}Controller) Cache() {{.type}}ControllerCache {
	return &{{.lowerName}}ControllerCache{
		lister:  c.informer.Lister(),
		indexer: c.informer.Informer().GetIndexer(),
	}
}

func (c *{{.lowerName}}Controller) Create(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).Create(obj)
}

func (c *{{.lowerName}}Controller) Update(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).Update(obj)
}

func (c *{{.lowerName}}Controller) UpdateStatus(obj *{{.version}}.{{.type}}) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}obj.Namespace{{end}}).UpdateStatus(obj)
}

func (c *{{.lowerName}}Controller) Delete({{ if .namespaced}}namespace, {{end}}name string, options *metav1.DeleteOptions) error {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Delete(name, options)
}

func (c *{{.lowerName}}Controller) DeleteCollection({{ if .namespaced}}namespace string, {{end}}options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).DeleteCollection(options, listOptions)
}

func (c *{{.lowerName}}Controller) Get({{ if .namespaced}}namespace, {{end}}name string, options metav1.GetOptions) (*{{.version}}.{{.type}}, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Get(name, options)
}

func (c *{{.lowerName}}Controller) List({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (*{{.version}}.{{.type}}List, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).List(opts)
}

func (c *{{.lowerName}}Controller) Watch({{ if .namespaced}}namespace string, {{end}}opts metav1.ListOptions) (watch.Interface, error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Watch(opts)
}

func (c *{{.lowerName}}Controller) Patch({{ if .namespaced}}namespace, {{end}}name string, pt types.PatchType, data []byte, subresources ...string) (result *{{.version}}.{{.type}}, err error) {
	return c.clientGetter.{{.plural}}({{ if .namespaced}}namespace{{end}}).Patch(name, pt, data, subresources...)
}

type {{.lowerName}}ControllerCache struct {
	lister  listers.{{.type}}Lister
	indexer cache.Indexer
}

func (c *{{.lowerName}}ControllerCache) Get({{ if .namespaced}}namespace, {{end}}name string) (*{{.version}}.{{.type}}, error) {
	return c.lister.{{ if .namespaced}}{{.plural}}(namespace).{{end}}Get(name)
}

func (c *{{.lowerName}}ControllerCache) List({{ if .namespaced}}namespace string, {{end}}selector labels.Selector) ([]*{{.version}}.{{.type}}, error) {
	return c.lister.{{ if .namespaced}}{{.plural}}(namespace).{{end}}List(selector)
}

func (c *{{.lowerName}}ControllerCache) AddIndexer(indexName string, indexer {{.type}}Indexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*{{.version}}.{{.type}}))
		},
	}))
}

func (c *{{.lowerName}}ControllerCache) GetByIndex(indexName, key string) (result []*{{.version}}.{{.type}}, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		result = append(result, obj.(*{{.version}}.{{.type}}))
	}
	return result, nil
}
`