package main

import (
	"fmt"
        iptables "github.com/coreos/go-iptables/iptables"
         "strings"
       "reflect"
         "regexp"
         

)


func contains(list []string, value string) bool {
	for _, val := range list {
		if val == value {
			return true
		}
	}
	return false
}

func generateChain(name string) string{
	if isIP(name){
		return strings.Replace(strings.Replace(name,".","_",-1),"/","_",-1)
	}else{
		return strings.Replace(name,".","_",-1)
	}
	
	return "MISCELLANEOUS"
}


var (
	ipv4Regex = regexp.MustCompile(`(?:[0-9]{1,3}\.){3}[0-9]{1,3}`)
	tagRegex  = regexp.MustCompile(`(?:[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?)`)
	ip4t *iptables.IPTables
	ipList []string
)

type route struct {
	route_chain   string
	Vm            string
	dns_ip_subnet string
	ext_public_ip string
	policy        string
	tags          []string
	network       string
	priority      int64
	zone          string
}

func isIP(ipaddr string) bool {
	if ipv4Regex.MatchString(ipaddr) {
		fmt.Printf("matcehd value  [%s]\n", ipaddr)
		return true
	}
	return false
}

/* 
Example multi routing chain as per gcp setup ason : 1Feb 2018.
Takes a route struct to process and generates chain on POSTROUTING chain. 
Adding all the rules for 1 domain to 1 chain, allowing easy management.
Do remeber to call first and last for POSTROUTING chain

First :  -A POSTROUTING -j LOG
-A POSTROUTING -d 196.11.240.223/32 -o eth0 -p tcp -j SNAT --to-source 35.192.234.183
-A POSTROUTING -d 65.49.33.90/32 -o eth0 -p tcp -j SNAT --to-source 35.192.234.183
-A POSTROUTING -d 54.194.94.237/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 65.49.33.70/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 34.251.229.121/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 54.72.118.87/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 52.17.63.228/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 52.30.59.195/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 34.252.166.237/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 34.240.190.76/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 52.212.158.217/32 -o eth0 -p tcp -j SNAT --to-source 35.194.7.216
-A POSTROUTING -d 40.68.96.177/32 -o eth0 -p tcp -j SNAT --to-source 104.198.225.55
-A POSTROUTING -d 41.178.51.21/32 -o eth0 -p tcp -j SNAT --to-source 104.198.225.55

Last: -A POSTROUTING -o eth0 -j MASQUERADE

*/
func iptablesUpdate(r route) {
	ip4t, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		fmt.Printf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
	}
        				fmt.Println(reflect.TypeOf(ip4t))

	
	chain_name := generateChain(r.route_chain)
//	Verify that chain exists
	list , err := ip4t.ListChains("nat")
	if err != nil {
		fmt.Printf("Issue while fetching the list for chain : %v", err)
	}
	
	if !contains(list, chain_name) {
		fmt.Printf("list doesn't contain the new chain %v, Lets create one", chain_name)
		er := ip4t.NewChain("nat", chain_name)
		if er != nil{
			fmt.Printf("Issue while creating chain %v", chain_name)
		}
	}
	
	 err1 := ip4t.AppendUnique("nat", chain_name , "-d", r.dns_ip_subnet, "-o", "eth0", "-p" ,"tcp", "-j", "SNAT", "--to-source",r.ext_public_ip)

	if err1 != nil {
		fmt.Printf("AppendUnique failed : %v", err)
	}
	
	error := ip4t.AppendUnique("nat", "POSTROUTING" , "-j",chain_name)
	if error != nil {
		fmt.Printf("AppendUnique failed : %v", err)
	}


	list, err3 := ip4t.List("nat", "POSTROUTING")
	if err3 != nil {
		fmt.Printf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
	}
	fmt.Println(list)

}

func main() {
	ip4t, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		fmt.Printf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
	}
fmt.Println(reflect.TypeOf(ip4t))



        out,_ :=ip4t.ListWithCounters("nat","POSTROUTING")
        for _, val := range out {
		fmt.Println(val)
	}
       fmt.Printf("values: %v", out)

        /*#err =  ip4t.AppendUnique("filter", "INPUT", "-s", "192.168.254.99", "-d", "127.0.0.1", "-j", "ACCEPT")

        #if err != nil {
        #        fmt.Printf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
        }*/

        list, err := ip4t.List("nat","POSTROUTING")
       	 if err != nil {
                fmt.Printf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
        }
        fmt.Println(list)


       r := route{
                        route_chain: "yahoo.com",
                        Vm: "self",
						dns_ip_subnet: "172.217.26.238",
						ext_public_ip: "127.0.0.1",
						policy:        "p1",
	}


     iptablesUpdate(r)
      verify()

}

func verify(){
	 ip4t, _ := iptables.NewWithProtocol(iptables.ProtocolIPv4)
       
	list, err := ip4t.List("nat", "POSTROUTING")
	if err != nil{
		fmt.Printf("ERROR while getting the list: %v",err)
	}
	
	fmt.Printf("List is %v:  ",list)
	listSize := len(list)
         fmt.Printf("List is %v:  ",listSize)
	for index, v := range list {
		if index == 1 {
			if v != "-A POSTROUTING -j LOG"{
				fmt.Printf("POSTROUTING chain's first line shoul be -A POSTROUTING -j LOG, but it is:%v",v )
			}
		}
	}	 
}



func generateChainName(name string) string{
	if isIP(name){
		return strings.Replace(strings.Replace(name,".","_",-1),"/","_",-1)
	}else{
		return strings.Replace(name,".","_",-1)
	}
}


func generateChain(name string) string{
	var cname string
	if isIP(name){
		cname =  strings.Replace(strings.Replace(name,".","_",-1),"/","_",-1)
	}else{
		cname =  strings.Replace(name,".","_",-1)
	}
	
	list , err := ip4t.ListChains("nat")
	if err != nil {
		fmt.Printf("Issue while fetching the list for chain : %v", err)
	}
	if !contains(list, cname) {
		fmt.Printf("list doesn't contain the new chain %v, Lets create one", cname)
		er := ip4t.NewChain("nat", cname)
		if er != nil{
			fmt.Printf("Issue while creating chain %v", cname)
		}
	}
	return cname
}
