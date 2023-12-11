// client create: ArtefactServiceClient
/*
  Created by /home/cnw/devel/go/yatools/src/golang.yacloud.eu/yatools/protoc-gen-cnw/protoc-gen-cnw.go
*/

/* geninfo:
   filename  : protos/golang.conradwood.net/apis/artefact/artefact.proto
   gopackage : golang.conradwood.net/apis/artefact
   importname: ai_0
   clientfunc: GetArtefactService
   serverfunc: NewArtefactService
   lookupfunc: ArtefactServiceLookupID
   varname   : client_ArtefactServiceClient_0
   clientname: ArtefactServiceClient
   servername: ArtefactServiceServer
   gsvcname  : artefact.ArtefactService
   lockname  : lock_ArtefactServiceClient_0
   activename: active_ArtefactServiceClient_0
*/

package artefact

import (
   "sync"
   "golang.conradwood.net/go-easyops/client"
)
var (
  lock_ArtefactServiceClient_0 sync.Mutex
  client_ArtefactServiceClient_0 ArtefactServiceClient
)

func GetArtefactClient() ArtefactServiceClient { 
    if client_ArtefactServiceClient_0 != nil {
        return client_ArtefactServiceClient_0
    }

    lock_ArtefactServiceClient_0.Lock() 
    if client_ArtefactServiceClient_0 != nil {
       lock_ArtefactServiceClient_0.Unlock()
       return client_ArtefactServiceClient_0
    }

    client_ArtefactServiceClient_0 = NewArtefactServiceClient(client.Connect(ArtefactServiceLookupID()))
    lock_ArtefactServiceClient_0.Unlock()
    return client_ArtefactServiceClient_0
}

func GetArtefactServiceClient() ArtefactServiceClient { 
    if client_ArtefactServiceClient_0 != nil {
        return client_ArtefactServiceClient_0
    }

    lock_ArtefactServiceClient_0.Lock() 
    if client_ArtefactServiceClient_0 != nil {
       lock_ArtefactServiceClient_0.Unlock()
       return client_ArtefactServiceClient_0
    }

    client_ArtefactServiceClient_0 = NewArtefactServiceClient(client.Connect(ArtefactServiceLookupID()))
    lock_ArtefactServiceClient_0.Unlock()
    return client_ArtefactServiceClient_0
}

func ArtefactServiceLookupID() string { return "artefact.ArtefactService" } // returns the ID suitable for lookup in the registry. treat as opaque, subject to change.

func init() {
   client.RegisterDependency("artefact.ArtefactService")
}


