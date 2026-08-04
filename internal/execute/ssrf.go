package execute

import (
	"fmt"
	"errors"
	"net/url"
	"net"
)


func SafeURL(raw string) (error){
	

	u, err := url.Parse(raw)

	if err != nil {
		return fmt.Errorf("Failed to parse URL: %v", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url scheme must be http or https")
	}

	if u.Host == ""{
		return errors.New("url host is invalid")
	}

	host := u.Hostname()

	ips, err := net.LookupIP(host)

	if err != nil {
		return fmt.Errorf("Failed to resolve host %s: %v", host, err)
	}

	if len(ips) == 0 {
		return errors.New("NO IPS found")
	} 


	for _, ip := range ips{
  		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified(){
    		return errors.New("url resolves to blocked address")
		}
	}
	return nil

}