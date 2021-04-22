module mysql-failover-leader

go 1.15

require (
	github.com/google/uuid v1.2.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.0.0-00010101000000-000000000000
	k8s.io/klog/v2 v2.8.0
)

replace k8s.io/client-go => k8s.io/client-go v0.19.10
