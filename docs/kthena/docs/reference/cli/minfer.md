## minfer

Kthena CLI for managing AI inference workloads

### Synopsis

minfer is a CLI tool for managing Kthena AI inference workloads.

It allows you to:
- Create manifests from predefined templates with custom values
- List and view Kthena resources in Kubernetes clusters
- Manage inference workloads, models, and autoscaling policies

Examples:
  minfer get templates
  minfer describe template deepseek-r1-distill-llama-8b
  minfer get template deepseek-r1-distill-llama-8b -o yaml
  minfer create manifest --name my-model --template deepseek-r1-distill-llama-8b
  minfer get models
  minfer get modelServingList --all-namespaces

### Options

```
  -h, --help     help for minfer
  -t, --toggle   Help message for toggle
```

### SEE ALSO

* [minfer create](minfer_create.md)	 - Create kthena resources
* [minfer describe](minfer_describe.md)	 - Show detailed information about a specific resource
* [minfer get](minfer_get.md)	 - Display one or many resources

