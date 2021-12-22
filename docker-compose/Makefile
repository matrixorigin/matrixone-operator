.PHONY: matrixone production start
prod-up:
	export IMAGE=matrixorigin/matrixone TAG=latest; docker-compose -f docker-compose.yml up -d

.PHONY: matrixone production stop
down:
	docker-compose -f docker-compose.yml down --remove-orphans

.PHONY: custom image version start
up:
	docker-compose -f docker-compose.yml up -d

.PHONY: clean data and log
clean:
	rm -rf rm -rf log* && rm -rf store*

.PHONY: clean log, data, repo
all-clean:
	rm -rf log* && rm -rf store* && rm -rf matrixone
 
.PHONY: matrixone dev clone and build
dev-pre:
	git clone git@github.com:matrixorigin/matrixone.git -b ${BRANCH} && cp Dockerfile matrixone
	
.PHONY: matrixone dev image build
dev-build:
	cd matrixone && docker build . -t matrixone:dc

.PHONY: matrixone dev cluster start
dev-up:
	export IMAGE=matrixone TAG=dc; docker-compose -f docker-compose.yml up -d

.PHONY: matrixone client start
mo-client:
	docker run -it --rm --name mo-client arey/mysql-client -h ${HOST} -P ${PORT} -udump -p111 
