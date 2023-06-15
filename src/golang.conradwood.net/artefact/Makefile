.PHONY:	client server
all:	client server
client:
	cd client && go install ${LDFLAGS} artefact-client.go `ls -1 *.go|grep -v artefact-client.go`
server:
	cd server && go install ${LDFLAGS} artefact-server.go `ls -1 *.go|grep -v artefact-server.go`
