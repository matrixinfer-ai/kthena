```plantuml
skinparam linetype polyline
skinparam component<<dashed>> {
  BorderStyle dashed
  BorderColor gray
  BackgroundColor white
}

actor operator
actor user

rectangle "Control Plane" as control_plane {
	component "k8s API Server" as k8s_api_server {
		component "CRD" as crd {
			component "Model" as model_crd
			component "ModelRoute" as modelroute_crd
			component "ModelServer" as modelserver_crd
			component "ModelInfer" as modelinfer_crd
			component "AutoScalingPolicy" as autoscalingpolicy_crd
			model_crd --> modelroute_crd
			model_crd --> modelinfer_crd
			model_crd --> autoscalingpolicy_crd
			modelroute_crd --> modelserver_crd
		}
	}
	
	rectangle "Network" as network {
		component "InferGateway Webhook (main.go)" as infergateway_webhook
		rectangle "Infer Gateway (main.go)" {
			component "Model Router" as modelrouter {
				component "Filter" as router_filter {
					component "Auth" as auth
					component "Rate Limiter" as rate_limiter
					auth --> rate_limiter
				}
				
				component "Router Proxy" as router_proxy
				component "Scheduler" as scheduler {
					component "Filter Plugins"
					component "Score Plugins"
					component "Pod Selector"
				}
				component "KV connector" as connector
				
				auth --> router_proxy
				router_proxy --> scheduler : schedule inference pods
				router_proxy --> connector : PD-Disaggregation mode
			}
			rectangle "Network Controllers" {
				component "ModelRouteController" as modelroute_controller
				component "ModelServerController" as modelserver_controller
			}

			database "In-Memory Datastore" as network_datastore {
				component "Model Route" as modelroute_data
				component "Model Server"
				component "Pods"
				component "RequestWaitingQueue" as request_waiting_queue
			}
			router_proxy --> request_waiting_queue : fairness
		}
		' layout
		infergateway_webhook -[hidden]-> modelrouter
	}


	rectangle "Registry" as registry {
		component "Registry Webhook (main.go)" as registry_webhook
		component "ModelController (main.go)" as model_controller
		component "AutoscalingPolicyController (main.go)" as autoscaling_controller {
			component "Optimizer" as autoscaler_optimizer {
				component "Metric Collector"
				component "Scaling Algorithm"
				component "Policy Engine"
			}
		}
		
		' layout
		registry_webhook -[hidden]-> model_controller
	}

	rectangle "Workload" {
		component "ModelInfer Webhook (main.go)" as modelinfer_webhook
		rectangle "Infer Controller (main.go)" as modelinfer_controller {
			database "Work Queue" as work_queue
			component "Worker" as worker {
				component "Manage Replica, Pod Lifecycle, Rolling Update, InferGroup"
			}
			
			database "In-Memory Datastore" as workload_datastore {
				component "InferGroups"
				component "Roles (Prefill/Decode)"
				component "Pod Status"
			}
			worker --> work_queue
			worker --> workload_datastore
		}
		
		' layout
		modelinfer_webhook -[hidden]-> modelinfer_controller
	}
}

rectangle "Data Plane" as data_plane {
	
	component "Inference Pods" as inference_pods {
		component "Group 0" as g0 {
			node "Role A " as g0ra {
				component "Entry Pod" as g0rae
				component "Worker Pod" as g0raw
				component "downloader" <<dashed>> as g0rad
				component "runtime (metrics)" <<dashed>> as g0rar
			}
			node "Role B" as g0rb {
				component "Entry Pod" as g0rbe
				component "downloader" <<dashed>> as g0rbd
				component "runtime (metrics)" <<dashed>> as g0rbr
			}
			node "Role N" as g0rn {
				component "Entry Pod" as g0rne
				component "Worker Pod" as g0rnw
				component "downloader" <<dashed>> as g0rnd
				component "runtime (metrics)" <<dashed>> as g0rnr
			}
		}
		
		component "Group 1" as g1 {
			node "Role A " as g1ra {
				component "Entry Pod" as g1rae
				component "Worker Pod" as g1raw
				component "downloader" <<dashed>> as g1rad
				component "runtime (metrics)" <<dashed>> as g1rar
			}
			node "Role B" as g1rb  {
				component "Entry Pod" as g1rbe
				component "downloader" <<dashed>> as g1rbd
				component "runtime (metrics)" <<dashed>> as g1rbr
			}
			node "Role N" as g1rn  {
				component "Entry Pod" as g1rne
				component "Worker Pod" as g1rnw
				component "downloader" <<dashed>> as g1rnd
				component "runtime (metrics)" <<dashed>> as g1rnr
			}
		}
	}
}

' request
operator ----> crd : create/update/delete
user ----> auth : infer request
router_proxy ----down---> inference_pods
network_datastore ------> inference_pods : periodically get metrics from backend (vLLM, sglang)
autoscaling_controller ------> inference_pods : periodically get metrics


' datastore
modelroute_controller --> network_datastore : sync
modelserver_controller --> network_datastore : sync
modelrouter --> network_datastore : consume
modelroute_data -up-> rate_limiter : callback


```