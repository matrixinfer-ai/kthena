## minfer get

Display one or many resources

### Synopsis

Display one or many resources.

You can get templates, models, modelinfers, and autoscaling policies.

Examples:
  minfer get templates
  minfer get template deepseek-r1-distill-llama-8b
  minfer get template deepseek-r1-distill-llama-8b -o yaml
  minfer get models
  minfer get models --all-namespaces
  minfer get modelinfers -n production

### Options

```
  -A, --all-namespaces     List resources across all namespaces
  -h, --help               help for get
  -n, --namespace string   Kubernetes namespace (default: current context namespace)
  -o, --output string      Output format (yaml|json|table)
```

### SEE ALSO

* [minfer](minfer.md)	 - MatrixInfer CLI for managing AI inference workloads
* [minfer get autoscaling-policies](minfer_get_autoscaling-policies.md)	 - List autoscaling policies
* [minfer get autoscaling-policy-bindings](minfer_get_autoscaling-policy-bindings.md)	 - List autoscaling policy bindings
* [minfer get modelinfers](minfer_get_modelinfers.md)	 - List model inference workloads
* [minfer get models](minfer_get_models.md)	 - List registered models
* [minfer get template](minfer_get_template.md)	 - Get a specific template
* [minfer get templates](minfer_get_templates.md)	 - List available manifest templates

