---
title: Authentication of infer gateway
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

Proposal for authentication to the infer gateway. Includes JWT and API Key.

### Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap.

A good summary is probably at least a paragraph in length.
-->

This document describes a proposal to introduce JWT (JSON Web Token) and API Key authentication mechanisms in Infer Gateway. The proposal aims to enhance the security of the Gateway by allowing users to configure different authentication methods for different model services, while maintaining the flexibility and scalability of the system.

### Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->

The current Infer Gateway lacks an authentication mechanism; any user with access to the gateway can invoke the model services, which poses the following problems:

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

1. Implement JWT authentication mechanism in Infer Gateway.
2. Implement API Key authentication mechanism in Infer Gateway.
3. Allow users to configure different authentication methods for different model services.
4. Support integration with external identity providers(e.g. Keycloak).
5. Provide flexible configuration options to meet the needs of different deployment environments.

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
2. The client uses the authentication token to invoke the Infer Gateway.
3. The Infer Gateway verifies the validity of the token.
4. If the authentication passes, the request is forwarded to the appropriate model service.
5. If authentication fails, a 401 Unauthorized Httpstatus Code is returned.

The infer gateway determines whether or not to validate the token based on the mod being accessed.

**Process of JWT:**

- Extract the `modelName` from the request, get the `modelServer` based on the `modelName`, and determine if the request needs to be authenticated.
- If need authenticated, extracting `Bearer Token` from Authorization header.
- Parse JWT header to get kid(key ID).
- Get corresponding public key from JWKS endpoint based on kid.
- Verify JWT signature using public key.
- Verify JWT declaration (expiration time, issuer, audience, etc.).
- Allow request to proceed if validation passes

Provide caching mechanism for jwks to improve the performance of JWT authentication.

```go
type JWKSProvider struct {
	config     *JWTConfig
	jwksSet    jwk.Set
	lastUpdate time.Time
	mutex      sync.RWMutex
}

type JWTConfig struct {
	IssuerURL    string `json:"issuerURL"` // jwks issuer URL
	Audience     string `json:"audience"`
	JWKSEndpoint string `json:"jwksEndpoint"`

	JWKSCacheDuration time.Duration `json:"jwkCacheDuration"` // default: 1 hour
}
```

- Default cache time: 1 hour.
- Support for configuring cache time.
- Automatically refreshes expired JWKS.
- Attempts to refetch JWKS on failure.

Process of API Key:

- Determine if `modelServer` needs authentication
- Extract API Key from configured location (Header or Query Parameter).
- Obtain list of valid API Keys based on configured source.
- Verify that extracted API Key is in valid list.
- Prevent timing attacks using time constant comparisons.
- Allow request to continue processing if validation passes.

#### CRD Extensions

JWT:

```go
type ModelServerSpec struct {
    // Existing fields...
    
    // NeedAuthenticate defines whether the model server requires Authentication.
    // +optional
    NeedAuthenticate bool `json:"needAuthenticate,omitempty"`

    // JWTAuthentication defines the JWT Authentication configuration.
    // +optional
    JWTAuthentication *JWTAuthenticationSpec `json:"jwtAuthentication,omitempty"`

    // APIKeyAuthentication defines the API Key Authentication configuration.
    // +optional
    APIKeyAuthentication *APIKeyAuthenticationSpec `json:"apiKeyAuthentication,omitempty"`
}

type JWTAuthenticationSpec struct {
    // IssuerURL is the URL of the JWT issuer (e.g., Keycloak realm URL).
    // +optional
    IssuerURL string `json:"issuerURL,omitempty"`
    
    // Audience is the expected audience of the JWT token.
    // +optional
    Audience string `json:"audience,omitempty"`
    
    // JWKSEndpoint is the endpoint to fetch JWKS keys.
    // If not specified, it will be constructed from IssuerURL.
    // +optional
    JWKSEndpoint string `json:"jwksEndpoint,omitempty"`
    
    // JWKSCacheDuration is the duration to cache JWKS keys.
    // +optional
    // +kubebuilder:default="1h"
    JWKSCacheDuration *metav1.Duration `json:"jwkCacheDuration,omitempty"`
}
```

API Key:

```go
type APIKeyauthenticationSpec struct {
    // Source defines where to get API keys from.
    // +kubebuilder:validation:Enum=inline;secret
    Source APIKeySource `json:"source"`
    
    // HeaderName is the HTTP header name to extract API key from.
    // +optional
    HeaderName string `json:"headerName,omitempty"`
    
    // QueryParam is the query parameter name to extract API key from.
    // +optional
    QueryParam string `json:"queryParam,omitempty"`
    
    // InlineKeys are API keys defined inline (for testing only).
    // +optional
    InlineKeys []string `json:"inlineKeys,omitempty"`
    
    // SecretName is the name of Kubernetes Secret containing API keys.
    // +optional
    SecretName string `json:"secretName,omitempty"`
}

type APIKeySource string

const (
    APIKeySourceInline APIKeySource = "inline"
    APIKeySourceSecret APIKeySource = "secret"
)
```

example:

```yaml
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: deepseek-r1-1-5b
  namespace: default
spec:
  model: "deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B"
  inferenceEngine: vLLM
  needauthentication: true
  jwtauthentication:
    issuerURL: "http://localhost:9999/auth/realms/dashboard"
    audience: "infer-gateway"
    jwkCacheDuration: "1h"
  workloadSelector:
    matchLabels:
      app: deepseek-r1-1-5b
  workloadPort:
    port: 8000
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: model-api-keys
  namespace: default
type: Opaque
data:
  key1: "c2stMTIzNDU2Nzg5MGFiY2RlZg=="  # base64 encoded "sk-1234567890abcdef"
  key2: "c2stZmVkY2JhMDk4NzY1NDMyMQ=="  # base64 encoded "sk-fedcba0987654321"

---
apiVersion: networking.matrixinfer.ai/v1alpha1
kind: ModelServer
metadata:
  name: deepseek-r1-1-5b
  namespace: default
spec:
  model: "deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B"
  inferenceEngine: vllm
  needauthentication: true
  apiKeyauthentication:
    source: "secret"
    secretName: "model-api-keys"
    headerName: "X-API-Key"
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