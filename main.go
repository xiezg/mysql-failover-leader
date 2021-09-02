/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func buildConfig(kubeconfig string) (*rest.Config, error) {

	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {

	klog.InitFlags(nil)

	var kubeconfig string
	var leaseLockName string
	var leaseLockNamespace string
	var id string
	var targetPID int

	flag.IntVar(&targetPID, "pid", os.Getppid(), "send single SIGUSR1 to pid")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&id, "id", uuid.New().String(), "the holder identity name")
	flag.StringVar(&leaseLockName, "lease-lock-name", "", "the lease lock resource name")
	flag.StringVar(&leaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	flag.Parse()

	if leaseLockName == "" {
		klog.Fatal("unable to get lease lock resource name (missing lease-lock-name flag).")
	}

	if leaseLockNamespace == "" {
		klog.Fatal("unable to get lease lock resource namespace (missing lease-lock-namespace flag).")
	}

	klog.Infof("id:%v", id)
	// leader election uses the Kubernetes API by writing to a
	// lock object, which can be a LeaseLock object (preferred),
	// a ConfigMap, or an Endpoints (deprecated) object.
	// Conflicting writes are detected and each client handles those actions
	// independently.
	config, err := buildConfig(kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}
	client := clientset.NewForConfigOrDie(config)

	// want to step down
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// listen for interrupts or the Linux SIGTERM signal and cancel
	// our context, which the leader election code will observe and
	// step down
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()

	// we use the Lease lock type since edits to Leases are less common
	// and fewer objects in the cluster watch "all Leases".
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id, // Identity is the unique string identifying a lease holder across all participants in an election.
		},
	}

	run := func(ctx context.Context) {
		// complete your controller loop here
		klog.Info("Controller loop...")

		ps, err := os.FindProcess(targetPID)
		if err != nil {
			klog.Errorf("FindProcess:[%d] fails. err:%v", targetPID, err)
			time.Sleep(1 * time.Second) //此处不等待直接cancel,会导致line:209 le.renew(ctx) 失败，无法释放lease资源
			cancel()
			return
		}

		//// Run starts the leader election loop
		//197 func (le *LeaderElector) Run(ctx context.Context) {
		//198     defer runtime.HandleCrash()
		//199     defer func() {
		//200         le.config.Callbacks.OnStoppedLeading()
		//201     }()
		//202
		//203     if !le.acquire(ctx) {
		//204         return // ctx signalled done
		//205     }
		//206     ctx, cancel := context.WithCancel(ctx)
		//207     defer cancel()
		//208     go le.config.Callbacks.OnStartedLeading(ctx)
		//209     le.renew(ctx)
		//210 }

		if err := ps.Signal(syscall.SIGUSR2); err != nil {
			klog.Errorf("Signal [%d] fails. err:%v", targetPID, err)
			time.Sleep(1 * time.Second) //此处不等待直接cancel,会导致line:209 le.renew(ctx) 失败，无法释放lease资源
			cancel()
			return
		}

		klog.Info("send notify SIGUSR2")
	}

	// use a Go context so we can tell the leaderelection code when we
	// start the leader election code loop
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock: lock,
		// IMPORTANT: you MUST ensure that any code you have that
		// is protected by the lease must terminate **before**
		// you call cancel. Otherwise, you could have a background
		// loop still running and another process could
		// get elected before your background loop finished, violating
		// the stated goal of the lease.
		ReleaseOnCancel: true,
		LeaseDuration:   20 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			//由goroutine来执行 go le.config.Callbacks.OnStartedLeading(ctx)
			OnStartedLeading: func(ctx context.Context) {
				// we're notified when we start - this is where you would
				// usually put your code
				klog.Infof("myself get leader elected: %v", id)
				run(ctx)
			},
			OnStoppedLeading: func() {
				// we can do cleanup here
				klog.Infof("leader lost: %s", id)
				os.Exit(1) //The program terminates immediately; deferred functions are not run.
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == id {
					// I just got the lock
					return
				}
				klog.Infof("OnNewLeader new leader elected: %s", identity)
			},
		},
	})
}
