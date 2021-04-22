mysql-failover-leader: main.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" mysql-failover-leader
	cp mysql-failover-leader ~/mysql-ha/image/mysql-ha/

.PHONY : clean
clean:
	rm -rf mysql-failover-leader
