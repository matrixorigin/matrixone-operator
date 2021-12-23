PLATFORM ?= linux/amd64
TAG ?= latest
IMAGE ?= matrixorigin/matrixone
REPO ?= git@github.com:matrixorigin/matrixone.git
BRANCH ?= main
PORT ?= 6001
HOST ?= 172.19.0.2

.PHONY: matrixone production start
up:
	export IMAGE=${IMAGE} TAG=${TAG}; docker-compose -f docker-compose.yml up -d

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