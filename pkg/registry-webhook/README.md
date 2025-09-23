# Registry Webhook

The registry webhook is a Kubernetes admission controller that provides validation and mutation capabilities for Model resources in the Kthena system. It consists of two main components: a validating webhook and a mutating webhook.

## Validation Rules

### Model Resource Validation
The validation webhook enforces the following rules for Model resources:

#### Backend Worker Type Validation

1. **vLLM, SGLang, MindIE backends**: Must have exactly one worker of type `server`
2. **vLLMDisaggregated backends**: All workers must be of type `prefill` or `decode`
3. **MindIEDisaggregated backends**: All workers must be of type `prefill`, `decode`, `controller`, or `coordinator` (not `server`)

#### Backend Replica Bounds Validation

- `minReplicas` cannot be greater than `maxReplicas` for any backend
- The sum of `maxReplicas` across all backends cannot exceed 1,000,000

#### Scale-to-Zero Grace Period Validation

- `scaleToZeroGracePeriod` cannot exceed 1800 seconds (30 minutes)
- `scaleToZeroGracePeriod` cannot be negative

#### Worker Image Validation

- Container image references cannot be empty or contain only whitespace
- Container image references cannot contain spaces
- Basic format validation is performed on image strings

#### Autoscaling Policy Validation

- Validates that referenced AutoscalingPolicy resources exist (currently skipped in implementation)
- Ensures proper scoping of autoscaling policies between model-level and backend-level configurations

#### Autoscaling Policy Scope Validation

- Enforces mutual exclusivity between model-level and backend-level autoscaling policy references
- Ensures consistent autoscaling configuration across the model specification

### Autoscaling Policy Binding Resource Validation

#### ScalingConfig and OptimizerConfig Validation
- Among ScalingConfig and OptimizerConfig, exactly one of them must be configured, and it is not allowed to configure neither or both.

## Default Values (Mutator Webhook)

The mutating webhook applies the following default values when certain conditions are met:

### When `AutoscalingPolicy` is set at the model level:

1. **ScaleToZeroGracePeriod**: Defaults to `30 seconds` for all backends that don't have this value explicitly set
2. **CostExpansionRatePercent**: Defaults to `200` if not explicitly set

These defaults are only applied when the model has an autoscaling policy reference configured, ensuring that autoscaling-related settings have sensible default values.

## Webhook Configuration

### Endpoints

- **Validation**:
    - `/validate-registry-volcano-sh-v1alpha1-model`
    - `/validate-registry-volcano-sh-v1alpha1-autoscalingpolicybinding`
- **Mutation**: `/mutate-registry-volcano-sh-v1alpha1-model`
- **Health Check**: `/healthz`

### Default Settings

- **Port**: 8443
- **Timeout**: 30 seconds
- **TLS**: Required (minimum TLS 1.2)
- **Failure Policy**: Fail
- **Reinvocation Policy**: IfNeeded (for mutating webhook)

## Extending the Webhooks

### Adding New Validation Rules

To add new validation rules to the validation webhook:

1. **Create a new validation function** in `pkg/registry-webhook/handlers/validator.go`:
   ```go
   func validateNewRule(model *workloadv1alpha1.Model) field.ErrorList {
       var allErrs field.ErrorList
       // Add your validation logic here
       // Use field.Invalid() to create validation errors
       return allErrs
   }
   ```

2. **Add the validation function** to the `validateModel` method:
   ```go
   func (v *ModelValidator) validateModel(model *workloadv1alpha1.Model) (bool, string) {
       // ... existing code ...
       allErrs = append(allErrs, validateNewRule(model)...)
       // ... rest of the method ...
   }
   ```

3. **Write tests** in `pkg/registry-webhook/handlers/validator_test.go` to cover your new validation logic.

### Adding New Default Values

To add new default values to the mutating webhook:

1. **Modify the `mutateModel` function** in `pkg/registry-webhook/handlers/mutator.go`:
   ```go
   func (m *ModelMutator) mutateModel(model *workloadv1alpha1.Model) {
       // ... existing code ...
       
       // Add your new default value logic
       if model.Spec.YourNewField == nil {
           model.Spec.YourNewField = &YourDefaultValue
       }
   }
   ```

2. **Write tests** in `pkg/registry-webhook/handlers/mutator_test.go` to verify your mutation logic.

### Adding Support for New Resources

To extend the webhooks to support additional resource types:

1. **Create new handler functions** following the pattern in `validator.go` and `mutator.go`
2. **Register new endpoints** in `pkg/registry-webhook/server/server.go`:
   ```go
   mux.HandleFunc("/validate-registry-volcano-sh-v1alpha1-newresource", newResourceValidator.Handle)
   mux.HandleFunc("/mutate-registry-volcano-sh-v1alpha1-newresource", newResourceMutator.Handle)
   ```
3. **Update webhook configurations** in the Helm charts to include the new resource types
4. **Add corresponding tests** for the new resource handlers

### Best Practices

1. **Use field.ErrorList** for validation errors to provide structured error messages
2. **Log important events** using `klog` for debugging and monitoring
3. **Handle edge cases** gracefully and provide clear error messages
4. **Write comprehensive tests** for both positive and negative scenarios
5. **Follow Kubernetes admission controller best practices** for webhook development
6. **Use deep copies** when mutating objects to avoid unintended side effects

### Testing

Run the existing tests to ensure your changes don't break existing functionality:

```bash
go test ./pkg/registry-webhook/handlers/...
```

For integration testing, you can deploy the webhook to a test cluster and verify the behavior with actual Model resources.