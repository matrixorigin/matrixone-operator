PLATFORM ?= linux/amd64
TAG ?= latest
IMAGE ?= matrixorigin/matrixone
PORT ?= 6001
HOST ?= 172.19.0.2
BRANCH ?= main
PWD = $(shell pwd)
REPO ?= git@github.com:matrixorigin/matrixone.git

.PHONY: matrixone production start
up:
	@echo "store/log path: ${PWD}"
	@echo "mo cluster ip: 172.19.0.2-172.19.0.4"
	@echo "mo cluster port: 6001"
	@echo "image ${IMAGE}:${TAG}"
	@echo "mapped ports: 6001-6003"
	@echo "connect: mysql -h <MACHINE_IP> -P <PORT> -udump -p111"
	@export IMAGE=${IMAGE} TAG=${TAG}; docker-compose -f docker-compose.yml up -d

.PHONY: matrixone production stop
down:
	docker-compose -f docker-compose.yml down --remove-orphans

.PHONY: clean data and log
clean:
	rm -rf rm -rf log* && rm -rf store*

.PHONY: clean log, data, repo
all-clean:
	rm -rf log* && rm -rf store* && rm -rf matrixone
 
.PHONY: matrixone dev clone and build
dev-pre:
	git clone ${REPO} -b ${BRANCH} && cp Dockerfile matrixone
	
.PHONY: matrixone dev image build
dev-build:
	cd matrixone && docker build --platform=${PLATFORM} . -t ${IMAGE}:${TAG}

.PHONY: matrixone client start
mo-client:
	@echo "HOST: ${HOST}, PORT: ${PORT}"
	@docker run -it --rm --name mo-client --network docker-compose_monet arey/mysql-client -h ${HOST} -P ${PORT} -udump -p111

.PHONY: mysql client
mysql-client:
	@echo "HOST: ${HOST}, PORT: ${PORT}"
	docker run -it --rm --name mysql-client  arey/mysql-client -h ${HOST} -P ${PORT} -udump -p111
