.PHONY: matrixone production start
pro-up:
	docker-compose -f docker-compose.yml up -d

.PHONY: matrixone production stop 
pro-down:
	docker-compose -f docker-compose.yml down --remove-orphans

.PHONY: matrixone production clean
pro-clean:
	rm -rf log* && rm -rf store*
 
.PHONY: matrixone dev clone and build
dev-pre:
	git clone git@github.com:matrixorigin/matrixone.git -b ${BRANCH} && cp Dockerfile matrixone
	
.PHONY: matrixone dev image build
dev-build:
	cd matrixone && docker build . -t matrixone:dev

.PHONY: matrixone dev start
dev-up:
	cd dev && docker-compose -f docker-compose-dev.yml up -d

.PHONY: matrixone dev stop
dev-down:
	cd dev && docker-compose -f docker-compose-dev.yml down --remove-orphans

.PHONY: matrixone dev clean
dev-clean:
	cd dev && rm -rf log-dev* && rm -rf store-dev* && rm -rf matrixone 

.PHONY: matrixone client start
mo-client:
	docker run -it arey/mysql-client -h ${HOST} -P ${PORT} -udump -p111

.PHONY: image cleam
img-clean:
	docker rmi matrixone:dev && docker rmi matrixorigin/matrixone:latest
