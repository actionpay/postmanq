package main

import (
	"fmt"
	"net"
	"strings"
)

func main() {
	//mxes, _ := net.LookupMX("gmail.com")
	mxes, _ := net.LookupMX("leeching.net")
	//for _, mx := range mxes {
	fmt.Println("-->", mxes[0].Host)
	serverName := seekRealServerName(mxes[0].Host)
	fmt.Println("<--", serverName)
	fmt.Println("---")
	//}
}

func seekRealServerName(hostname string) string {
	parts := strings.Split(hostname, ".")
	partsLen := len(parts)
	lookupHostname := strings.Join(parts[partsLen-3:partsLen-1], ".")
	mxes, err := net.LookupMX(lookupHostname)
	if err == nil {
		if strings.Contains(mxes[0].Host, lookupHostname) {
			return hostname
		} else {
			return seekRealServerName(mxes[0].Host)
		}
	} else {
		return hostname
	}
}
