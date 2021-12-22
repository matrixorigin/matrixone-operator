.PHONY: matrixone production start
up:
	docker-compose -f docker-compose.yml up -d

.PHONY: matrixone production stop 
down:
	docker-compose -f docker-compose.yml down --remove-orphans

.PHONY: matrixone production clean
clean:
	rm -rf log* && rm -rf store* && rm -rf matrixone
 
.PHONY: matrixone dev clone and build
dev-pre:
	git clone git@github.com:matrixorigin/matrixone.git -b ${BRANCH} && cp Dockerfile matrixone
	
.PHONY: matrixone dev image build
dev-build:
	cd matrixone && docker build . -t matrixone:dc && export IMAGE=matrixone TAG=dc

.PHONY: matrixone client start
mo-client:
	docker run -it --rm --name mo-client arey/mysql-client -h ${HOST} -P ${PORT} -udump -p111 
