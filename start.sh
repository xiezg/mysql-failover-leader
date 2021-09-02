#########################################################################
# File Name: start.sh
# Author: xiezg
# mail: xzghyd2008@hotmail.com
# Created Time: 2021-06-03 18:28:27
# Last modified: 2021-09-02 20:53:28
#########################################################################
#!/bin/bash

if [ ./main.go -nt ./mysql-failover-leader ]
then
go build .
fi

leader=false

trap "leader=true" SIGUSR2 

./mysql-failover-leader -v 1 -kubeconfig=./config -logtostderr=true -lease-lock-name=test-mysql-failover -lease-lock-namespace=infra -pid $$ -id $1 &

while ! $leader
do
echo "`date`: sleep for leader"
sleep 3
done

while true
do
    sleep 3
    echo 'success'
done
