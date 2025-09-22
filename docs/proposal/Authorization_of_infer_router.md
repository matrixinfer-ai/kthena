---
title: Authentication of infer router
authors:
- "@LiZhenCheng9527" # Authors' GitHub accounts here.
reviewers:
- TBD
approvers:
- TBD

creation-date: 2025-08-04

---

## Your short, descriptive title

<!--
This is the title of your KEP. Keep it short, simple, and descriptive. A good
title can help communicate what the KEP is and should be considered as part of
any review.
-->

Proposal for authentication to the infer router. Includes JWT and API Key.

### Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap.

A good summary is probably at least a paragraph in length.
-->

This document describes a proposal to introduce JWT (JSON Web Token) and API Key authentication mechanisms in Infer Router. The proposal aims to enhance the security of the Router by allowing users to configure different authentication methods for different model services, while maintaining the flexibility and scalability of the system.

### Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->

The current Infer Router lacks an authentication mechanism; any user with access to the router can invoke the model services, which poses the following problems:

- **Security risk**: Unauthorized users may misuse model services.
- **Resource misuse**: Lack of access control may lead to overuse of resources.
- **Billing difficulties**: Inability to accurately track resource usage by different users.
- **Compliance Issues**: Enterprise-level deployments need to comply with security and compliance requirements.

By introducing JWT and API Key authentication, we can:

- Control access to model services.
- Implement user authentication and authentication.
- Provide different access rights for different users.
- Support integration with existing identity providers(e.g. Keycloak).

#### Goals

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

1. Implement JWT authentication mechanism in Infer Router.
2. Implement API Key authentication mechanism in Infer Router.
3. Support integration with external identity providers(e.g. Keycloak).

#### Non-Goals

<!--
What is out of scope for this KEP? Listing non-goals helps to focus discussion
and make progress.
-->

1. Does not implement OAuth2 authentication process(only validates acquired tokens).
2. Does not implement user management functionality(handled by an external identity provider).
3. Does not implement a complex role and permission management system.

### Proposal

<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

#### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

##### Story 1

##### Story 2

#### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go in to as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

#### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate?

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

### Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

#### Process

1. The client requests an authentication token from an identity provider(e.g. Keycloak).
2. The client uses the authentication token to invoke the Infer Router.
3. The Infer Router verifies the validity of the token.
4. If the authentication passes, the request is forwarded to the appropriate model service.
5. If authentication fails, a 401 Unauthorized Httpstatus Code is returned.

The infer router determines whether or not to validate the token based on the mod being accessed.

**Process of JWT:**

- Extracting `Bearer Token` from Authorization header.
- Parse JWT header to get kid(key ID).
- Verify JWT signature using public key.
- Verify JWT declaration (expiration time, issuer, audience, etc.).
- Allow request to proceed if validation passes

The above is the processing of getting jwsks to do the validation locally.

Provide caching mechanism for jwks to improve the performance of JWT authentication. Storing jwks as `Jwks` in store.

Since we have designed a cache for jwks, the create, delete, update, and retrieve related implementations of this cache should also be completed.

```go
type JWTValidator struct {
  enable bool
  cache  datastore.Store
}

type Jwks struct {
  Jwks      jwk.Set
  Audiences []string
  Issuer    string
  // Used to update jwks
  Uri         string
  ExpiredTime time.Duration
}

type store struct {
  jwksCache   *Jwks
  modelServer sync.Map // map[types.NamespacedName]*modelServer
  pods        sync.Map // map[types.NamespacedName]*PodInfo
  // ......
}
```

We are using configMap to configure JWT Authentication at this stage.

```yaml
auth:
  issuer: "https://secure.istio.io"
  audiences: ["matrixinfer.io"]
  jwksUri: "https://raw.githubusercontent.com/istio/istio/release-1.27/security/tools/jwt/samples/jwks.json"
```

**Process of API Key:**

- Determine if `modelServer` needs authentication
- Extract API Key from configured location (Header or Query Parameter).
- Obtain list of valid API Keys based on configured source.
- Verify that extracted API Key is in valid list.
- Prevent timing attacks using time constant comparisons.
- Allow request to continue processing if validation passes.

#### CRD Extensions

```go
type ModelServerSpec struct {
    // APIKeyRules define the API Key Authentication configuration.
    // +optional
    APIKeyRules []APIKeyRuls `json:"apiKeyAuthentication,omitempty"`
}

type APIKeyRules struct {
    // FromHeader is the HTTP header name to extract API key from.
    // +optional
    FromHeader string`json:"fromHeader,omitempty"`
    
    // FromParam is the query parameter name to extract API key from.
    // +optional
    FromParam string `json:"fromParam,omitempty"`
    
    // SecretName is the name of Kubernetes Secret containing API keys.
    // Only one of labelSelector and secretName should be specified. When both are specified, labelSelector takes precedence.
    // +optional
    SecretName types.NamespacedName `json:"secretName,omitempty"`

    // labelSelector is the label selector to filter API keys.
    // Only one of labelSelector and secretName should be specified. When both are specified, labelSelector takes precedence.
    // +optional
    labelSelector map[string]string `json:"labelSelector,omitempty"`
}
```

You can use the `labelSelector` to specify the required `secrets`.

#### Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: model-api-keys
  namespace: default
type: Opaque
data:
  api_key1: "c2stMTIzNDU2Nzg5MGFiY2RlZg=="  # base64 encoded "sk-1234567890abcdef"
  api_key2: "c2stZmVkY2JhMDk4NzY1NDMyMQ=="  # base64 encoded "sk-fedcba0987654321"

---
apiVersion: networking.volcano.sh/v1alpha1
kind: ModelServer
metadata:
  name: deepseek-r1-1-5b
  namespace: default
spec:
  model: "deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B"
  inferenceEngine: vLLM
  apiKeyRules:
  - secretName:
      name: "model-api-keys"
      namespace: "default"
    fromHeader: "X-API-Key"
  workloadSelector:
    matchLabels:
      app: deepseek-r1-1-5b
  workloadPort:
    port: 8000
```

#### Test Plan

<!--
**Note:** *Not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all test cases, just the general strategy. Anything
that would count as tricky in the implementation, and anything particularly
challenging to test, should be called out.

-->

### Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->