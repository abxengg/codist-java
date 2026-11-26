package java

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// Calls between services. A Feign client in one service calls another service of the same repository by its name;
// when that service serves no route with the client's verb and path, every such call fails with 404 or 405 at run
// time. The model reads one service's change at a time and cannot see the other side (shop-lab: a client renamed
// /payments/charge to /payments/charges and nothing was reported), so this is an exact check: the index already
// knows both sides. A client whose service is not in the repository is never reported.

// serviceOf is the service a source file belongs to: the folder that holds its src/main/java, as a Feign client
// names it ("payment-service"), or "" for a file outside that layout.
func serviceOf(file string) string {
	i := strings.Index(file, "/src/main/java/")
	if i <= 0 {
		return ""
	}
	return path.Base(file[:i])
}

// servedBy maps each service to the routes its controllers serve, from every main file of the project.
func servedBy(files []*File) map[string][]Endpoint {
	out := map[string][]Endpoint{}
	for _, f := range files {
		s := serviceOf(f.Path)
		if s == "" {
			continue
		}
		for _, t := range f.Types {
			if t.Feign == "" && t.Stereotype == "controller" {
				out[s] = append(out[s], t.Endpoints...)
			}
		}
	}
	return out
}

// routeMatches says a served route answers a call: the same verb (or one of them takes any), and the same path, a
// {variable} on either side standing for any one segment.
func routeMatches(served, call Endpoint) bool {
	if served.Verb != call.Verb && served.Verb != "ANY" && call.Verb != "ANY" {
		return false
	}
	a, b := strings.Split(strings.Trim(served.Path, "/"), "/"), strings.Split(strings.Trim(call.Path, "/"), "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !strings.HasPrefix(a[i], "{") && !strings.HasPrefix(b[i], "{") {
			return false
		}
	}
	return true
}

// unservedCalls reports each route of a Feign client in this file that its service, found in the repository, does
// not serve.
func (c *checker) unservedCalls(t Type) {
	served, known := c.facts.Served[t.Feign]
	if t.Feign == "" || !known || len(served) == 0 {
		return
	}
	for _, call := range t.Endpoints {
		if slices.ContainsFunc(served, func(s Endpoint) bool { return routeMatches(s, call) }) {
			continue
		}
		var routes []string
		for _, s := range served {
			routes = append(routes, s.Verb+" "+s.Path)
		}
		slices.Sort(routes)
		if len(routes) > 6 {
			routes = append(routes[:6], "…")
		}
		c.addAt(call.Line, call.Line, "unserved-call", "high", fmt.Sprintf("%s does not serve %s %s", t.Feign, call.Verb, call.Path),
			fmt.Sprintf("This client calls %s %s on %s, which serves no such route, so every call fails with 404 or 405. %s serves: %s.",
				call.Verb, call.Path, t.Feign, t.Feign, strings.Join(routes, ", ")),
			"Call a route the other service serves, or add the route there in the same change.")
	}
}
