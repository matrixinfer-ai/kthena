package controller

type Config struct {
	EnableLeaderElection bool
	Workers              int
	Kubeconfig           string
	MasterURL            string
}
