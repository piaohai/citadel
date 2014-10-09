package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/citadel/citadel"
	"github.com/citadel/citadel/cluster"
	"github.com/docker/docker/pkg/log"
	"github.com/gorilla/mux"
	"github.com/samalba/dockerclient"
)

type HttpApiFunc func(c *cluster.Cluster, w http.ResponseWriter, r *http.Request)

func ping(c *cluster.Cluster, w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{'O', 'K'})
}

func postContainersCreate(c *cluster.Cluster, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var image citadel.Image
	var config dockerclient.ContainerConfig

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		fmt.Println("Create Error:", err)
	}

	image.Name = config.Image
	image.Args = config.Cmd
	image.Type = "service"
	image.ContainerName = r.Form.Get("name")

	container, err := c.Create(&image, true)
	if err == nil {
		fmt.Fprintf(w, "{%q:%q}", "Id", container.ID)
	} else {
		fmt.Println("Create Error:", err)
	}
}

func postContainersStart(c *cluster.Cluster, w http.ResponseWriter, r *http.Request) {
	var image citadel.Image
	var config dockerclient.HostConfig

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		fmt.Println("Start Error1:", err)
	}

	container := c.ContainerByID(mux.Vars(r)["name"])

	if container != nil {
		if err := c.Start(container, &image); err == nil {
			fmt.Fprintf(w, "{%q:%q}", "Id", container.ID)
		} else {
			fmt.Println("Start Error2:", err)
		}
	}
}

func postContainersRestart(c *cluster.Cluster, w http.ResponseWriter, r *http.Request) {
	container := c.ContainerByID(mux.Vars(r)["name"])

	if container != nil {
		newURL, _ := url.Parse(container.Engine.Addr)
		newURL.RawQuery = r.URL.RawQuery
		newURL.Path = r.URL.Path
		fmt.Println("REDIR ->", newURL.String())
		http.Redirect(w, r, newURL.String(), 302)
	}
}

func getContainersJSON(c *cluster.Cluster, w http.ResponseWriter, r *http.Request) {
	var (
		err              error
		containers       []*citadel.Container
		dockerContainers []dockerclient.Container
	)

	if containers, err = c.ListContainers(); err != nil {
		log.Errorf("Failed to list containers: %v", err)
		return
	}

	for _, cs := range containers {
		dockerContainers = append(dockerContainers, citadel.ToDockerContainer(cs))
	}

	b, _ := json.Marshal(dockerContainers)
	w.Write(b)
}

func createRouter(c *cluster.Cluster) (*mux.Router, error) {
	r := mux.NewRouter()
	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/_ping": ping,
			//#			"/events": getEvents,
			//			"/info":                           getInfo,
			//#			"/version": getVersion,
			//			"/images/json":                    getImagesJSON,
			//			"/images/viz":                     getImagesViz,
			//			"/images/search":                  getImagesSearch,
			//			"/images/get":                     getImagesGet,
			//			"/images/{name:.*}/get":           getImagesGet,
			//			"/images/{name:.*}/history":       getImagesHistory,
			//			"/images/{name:.*}/json":          getImagesByName,
			"/containers/ps":   getContainersJSON,
			"/containers/json": getContainersJSON,
			//			"/containers/{name:.*}/export":    getContainersExport,
			//			"/containers/{name:.*}/changes":   getContainersChanges,
			//#			"/containers/{name:.*}/json": getContainersByName,
			//			"/containers/{name:.*}/top":       getContainersTop,
			//			"/containers/{name:.*}/logs":      getContainersLogs,
			//			"/containers/{name:.*}/attach/ws": wsContainersAttach,
		},
		"POST": {
			//			"/auth":                         postAuth,
			//			"/commit":                       postCommit,
			//			"/build":                        postBuild,
			//			"/images/create":                postImagesCreate,
			//			"/images/load":                  postImagesLoad,
			//			"/images/{name:.*}/push":        postImagesPush,
			//			"/images/{name:.*}/tag":         postImagesTag,
			"/containers/create": postContainersCreate,
			//# "/containers/{name:.*}/kill": postContainersKill,
			//#			"/containers/{name:.*}/pause":   postContainersPause,
			//#			"/containers/{name:.*}/unpause": postContainersUnpause,
			"/containers/{name:.*}/restart": postContainersRestart,
			"/containers/{name:.*}/start":   postContainersStart,
			//#"/containers/{name:.*}/stop":    postContainersStop,
			//			"/containers/{name:.*}/wait":    postContainersWait,
			//			"/containers/{name:.*}/resize":  postContainersResize,
			//			"/containers/{name:.*}/attach":  postContainersAttach,
			//			"/containers/{name:.*}/copy":    postContainersCopy,
			//			"/containers/{name:.*}/exec":    postContainerExecCreate,
			//			"/exec/{name:.*}/start":         postContainerExecStart,
			//			"/exec/{name:.*}/resize":        postContainerExecResize,
		},
		//#		"DELETE": {
		//#			"/containers/{name:.*}": deleteContainers,
		//			"/images/{name:.*}":     deleteImages,
		//#		},
		//		"OPTIONS": {
		//			"": optionsHandler,
		//		},
	}

	for method, routes := range m {
		for route, fct := range routes {
			log.Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localFct := fct
			wrap := func(w http.ResponseWriter, r *http.Request) {
				fmt.Printf("-> %s %s\n", r.Method, r.RequestURI)
				localFct(c, w, r)
			}
			localMethod := method

			// add the new route
			r.Path("/v{version:[0-9.]+}" + localRoute).Methods(localMethod).HandlerFunc(wrap)
			r.Path(localRoute).Methods(localMethod).HandlerFunc(wrap)
		}
	}

	return r, nil
}

func ListenAndServe(c *cluster.Cluster, addr string) error {
	r, err := createRouter(c)
	if err != nil {
		return err
	}
	s := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	return s.ListenAndServe()
}