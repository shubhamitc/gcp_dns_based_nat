package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"bytes"

	iptables "github.com/coreos/go-iptables/iptables"
	"github.com/tidwall/gjson"

	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var data = []byte(`
    {
        "nat-vm-1": [
            {
                "dns_ip_subnet": "yahoo.com",
                "ext_public_ip": "65.49.33.90",
                "policy": "p1",
                "tags": ["http-server"],
                "network": "https://www.googleapis.com/compute/v1/projects/premium-poc/global/networks/vuclip-poc",
                "zone": "asia-east1-a",
                "priority": "20000"
            },
            {
                "dns_ip_subnet": "google.com",
                "ext_public_ip": "65.49.33.90",
                "policy": "p2",
                "tags": ["https-server"],
                "network": "https://www.googleapis.com/compute/v1/projects/premium-poc/global/networks/vuclip-poc",
                "zone": "asia-east1-a",
                "priority": "20000"
            },
        ],
        "client1": {
                "id": "2212fw1",
                "name": "Papupapa Hernandez",
                "email": "papupapa@gmail.com",
                "phones": ["554-223-2311", "332-232-2123"]
        }
    }
`)

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
	baseRoute 	bool
}

var (
	ipv4Regex = regexp.MustCompile(`(?:[0-9]{1,3}\.){3}[0-9]{1,3}`)
	tagRegex  = regexp.MustCompile(`(?:[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?)`)
	ip4t *iptables.IPTables
	ipList []string
//	arecord := prometheus.NewCounter(prometheus.CounterOpts{
//			Name: "arecord_changed",
//			Help: "A record changed after dns evaluation"
//	})	

	arecord prometheus.Counter
	ggauge *prometheus.GaugeVec
)

func iterate(Vm string, data interface{}) []route {
	var ROUTES []route

	// fmt.Println(ROUTES)

	fmt.Printf("key[%s] value[%s]\n", Vm, data)
	if rec, ok := data.([]interface{}); ok {
		for key, val := range rec {
			log.Printf(" [========>] %s = %s", key, val)
			if _data, _ok := val.(map[string]interface{}); _ok {
				// Process tags
				aInterface := _data["tags"].([]interface{})
				var aString []string
				for _, v := range aInterface {
					// Validate tag regex pattern.
					if tagRegex.MatchString(v.(string)) {
						aString = append(aString, v.(string))
					}
				}
				// fmt.Println(aString)
				fmt.Println(reflect.TypeOf(aString))
				var addrs []string
				addrs = dns_lookup(convert_string(_data["dns_ip_subnet"]))
				for _, ip_value := range addrs {
					p, _ := strconv.ParseInt(convert_string(_data["priority"]), 10, 64)
					fmt.Println(p)
					r := route{
                        route_chain: convert_string(_data["dns_ip_subnet"]),
                        Vm: Vm,
						dns_ip_subnet: ip_value,
						ext_public_ip: convert_string(_data["ext_public_ip"]),
						policy:        convert_string(_data["policy"]),
						network:       convert_string(_data["network"]),
						zone:          convert_string(_data["zone"]),
						priority:      p,
						tags:          aString}
					fmt.Println("ROUTES ADDED", r)
					ROUTES = append(ROUTES, r)
				}
			}
		}
	} else {
		fmt.Printf("record not a map[string]interface{}: %v\n", data)
	}

	return ROUTES
}

func convert_string(i interface{}) string {
	str, ok := i.(string)
	if ok {
		return str
	}
	return ""
}

func isIP(ipaddr string) bool {
	if ipv4Regex.MatchString(ipaddr) {
		fmt.Printf("matcehd value  [%s]\n", ipaddr)
		return true
	}
	return false
}

func dns_lookup(r string) []string {
	fmt.Println("DNS query for :", r)
	var addrs []string
	var taddr []string
	if isIP(r) {
		taddr = append(taddr, r)
		return taddr
	}
	addrs, _ = net.LookupHost(r)
	fmt.Printf("ALL DNS(%s) value  [%s]\n", r, addrs)
	for _, addr := range addrs {
		if isIP(addr) {
			// fmt.Printf("IP matched from all is [%s]\n", addr)
			taddr = append(taddr, addr)
		}
	}
	return taddr
}

func get_or_create_route(service *compute.Service, project *string, _r route) {
	fmt.Println("Creating route: ", _r)

	route_name := _r.Vm + "-" + strings.Replace(_r.dns_ip_subnet, ".", "-", -1) + "-" + _r.policy

	resp, err := service.Routes.Get(*project, route_name).Do()
	if err != nil {
		fmt.Println("ERROR", err, strings.Join(_r.tags, "$"))

		rb := &compute.Route{
			DestRange:       _r.dns_ip_subnet,
			Name:            route_name,
			Network:         _r.network,
			Priority:        _r.priority,
			Tags:            _r.tags,
			NextHopInstance: "https://www.googleapis.com/compute/v1/projects/" + *project + "/zones/" + _r.zone + "/instances/" + _r.Vm,
		}

		_resp, _err := service.Routes.Insert(*project, rb).Do()
		if _err != nil {
			fmt.Println(_err)
			// log.Fatal(_err)
		}

		// TODO: Change code below to process the `resp` object:
		fmt.Printf("%#v\n", _resp)

		// Create
		// log.Fatal(err)
	}
	fmt.Println("RESP", resp)
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

/*
	Generic method to match a value in array/list.
*/
func contains(list []string, value string) bool {
	for _, val := range list {
		if val == value {
			return true
		}
	}
	return false
}

// Insert inserts the value into the slice at the specified index,
// which must be in range.
// The slice must have room for the new element.
func Insert(slice []string, index int, value string) []string {
    // Grow the slice by one element.
    fmt.Println("INSERT1",len(slice) , index)
    slice=append(slice, "")
    //slice = slice[0 : len(slice)+1]
    // Use copy to move the upper part of the slice out of the way and open a hole.
    fmt.Println("INSERT",len(slice) , index, cap(slice))

    copy(slice[index+1:], slice[index:])
    // Store the new value.
    slice[index] = value
    // Return the result.
    return slice
}

func appendToPostrouting(chain_name string) error {
//	err = ip4t.AppendUnique("nat", "POSTROUTING" , "-j", chain_name)
	exists, err := ip4t.Exists("nat", "POSTROUTING", "-j", chain_name)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString("-A POSTROUTING -h ")
	b.WriteString(chain_name)

	if !exists {
		if len(ipList) == 2 {
			fmt.Println("Inserting : len =2 ",len(ipList),b.String())
			ipList =  Insert(ipList, 2, b.String() )
			return ip4t.Insert("nat", "POSTROUTING", 2, "-j", chain_name)
		}else{
			fmt.Println("Insertign len != 2 ",len(ipList),b.String(),":====",ipList, cap(ipList))
			ipList =  Insert(ipList, (len(ipList)-2) ,b.String() )
			return ip4t.Insert("nat", "POSTROUTING",(len(ipList)-2), "-j", chain_name)
		}

	}
	return nil
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
func iptablesUpdate(r route,chain_name string) error {
	exists, err := ip4t.Exists("nat", chain_name,  "-d", r.dns_ip_subnet, "-o", "eth0", "-p" ,"tcp", "-j", "SNAT", "--to-source",r.ext_public_ip)
	if err != nil {
		return err
	}

	if !exists {
		arecord.Inc()
		ggauge.With(prometheus.Labels{"dns": r.route_chain, "newip": r.dns_ip_subnet}).Inc()
		return ip4t.Append("nat", chain_name,  "-d", r.dns_ip_subnet, "-o", "eth0", "-p" ,"tcp", "-j", "SNAT", "--to-source",r.ext_public_ip)
	}

	return nil
	
//	err := ip4t.AppendUnique("nat", chain_name , "-d", r.dns_ip_subnet, "-o", "eth0", "-p" ,"tcp", "-j", "SNAT", "--to-source",r.ext_public_ip)

//	if err != nil {
//		fmt.Printf("AppendUnique failed : %v", err)
//	}

}

func iptablesfirstLine() error {
	fmt.Println("Inserting first line")
	var b bytes.Buffer
	b.WriteString("-t nat -A POSTROUTING -j LOG")
	ipList = append(ipList, b.String())
//	ipList =  Insert(ipList, 0, b.String() )
	return ip4t.Insert("nat", "POSTROUTING", 1, "-j","LOG")
}


func iptablesLastLine(position int) error {
	fmt.Println("Insertign last line")
	var b bytes.Buffer
	b.WriteString("-t nat -A POSTROUTING  -o eth0 -j MASQUERADE")
	ipList = append(ipList, b.String())
	//ipList =  Insert(ipList, position, b.String() )
	return ip4t.Insert("nat", "POSTROUTING", position, "-o" ,"eth0", "-j", "MASQUERADE")
}

func iptablesDeleteLine(position int) error {
	return ip4t.Insert("nat", "POSTROUTING", position, "-o" ,"eth0", "-j", "MASQUERADE")
}

func computeService(projectid *string, credPath *string) *compute.Service {
	// # Set google cred context
	// projectId := *projectid
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", *credPath)

	ctx := context.Background()
	client, err := google.DefaultClient(ctx,
		compute.CloudPlatformScope,
	)
	if err != nil {
		log.Fatal(err)
	}

	service, err := compute.New(client)
	fmt.Println("TYPE:", ctx)
	if err != nil {
		log.Fatalf("Unable to create Compute service: %v", err)
	}
	return service
}

func readFile(file string) []byte {
	b, err := ioutil.ReadFile(file) // just pass the file name
	if err != nil {
		fmt.Print(err)
		log.Fatal("not able to read file")
	}
	return b

	// return gjson.Parse(string(b)).Value().(map[string]interface{})

}

func main() {
	// dns_lookup("192.168.254.9/32")

	r_context := flag.String("context", "iptables", "Either iptables|google_route")
	fetch := flag.String("fetchpath", "jsonfile", "Either path of json file or route metadata route from google.")
	projectid := flag.String("projectid", "", "projectid in which you are planning to add route.")
	google_cred := flag.String("google_cred", "GOOGLE_APPLICATION_CREDENTIALS", "projectid in which you are planning to add route.")
//	flush_postrouting := flag.Bool("flush", false, "Flush postrouting chain.")
	vmIptables := flag.String("vm", "", "VM name for which rules should be picked for. ")
	addr := flag.Int("listen-address", 8080, "The address to listen on for HTTP requests.")
	flag.Parse()
	
	arecord = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "a_record_changed", // Note: No help string...
		Help: "A record changed for some DNS.",
	})
	prmerror := prometheus.Register(arecord)
	if prmerror != nil {
		fmt.Println("Push counter couldn't be registered, no counting will happen:", prmerror)
	}
	
	ggauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ip_changed_to",
			Help: "Ip changed to this new ip",
			Subsystem: "runtime",
			},
		[]string{
			"dns",
			"newip",
		},
	)
	
//	ggauge.With(prometheus.Labels{"dns": "google", "newip": "127.0.0.1"}).Inc()
	fmt.Println(reflect.TypeOf(ggauge), addr)
	
	
	prmerror = prometheus.Register(ggauge)
	if prmerror != nil {
		fmt.Println("Push GaugeV couldn't be registered, no monitoring will happen:", prmerror)
	}


	var service *compute.Service

	if *r_context == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *fetch == "" {
		flag.PrintDefaults()
		log.Fatal("No json input is found for iptables. We need to have in format ", data)
	}

	if *r_context == "iptables" {
		log.Println("iptable context is provided. Staring...")
		// log.Println("Exiting after file processing.")
		// os.Exit(0)
	} else {
		if *projectid == "" {
			flag.PrintDefaults()
			log.Fatal("projectid is required when *context is not google_route")
		}
		service = computeService(projectid, google_cred)

	}

	// projectId := "premium-poc"
	// data_file := *fetch
	
	http.Handle("/metrics", promhttp.Handler())
	server := &http.Server{}
	listener, errListen := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: *addr})


	if errListen != nil {
	 log.Fatal("error creating listener")
	}
	
	
	
	
//	go server.Handler("/metrics", promhttp.Handler())
	go server.Serve(listener)
	
	// Comment for prod only here for testing
	//var e int

    //fmt.Scanf("%d", &e)
    //fmt.Fprintf(os.Stdout, "%#X\n", e)
	// comment till here
	
	for {
		var e int
	fmt.Scanf("%d", &e)
    fmt.Fprintf(os.Stdout, "%#X\n", e)
		b := readFile(*fetch)

		// Parse file to map
		result := gjson.Parse(string(b)).Value().(map[string]interface{})

		fmt.Printf("%+v\n", result)

		var vm_host_record []route
		for k, v := range result {
			_v := iterate(k, v)
			vm_host_record = append(vm_host_record, _v...)

		}

		var lastcname string
		var cname string
		if *r_context == "iptables" {
				var err error
				ip4t , err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
				if err != nil {
					fmt.Printf("NewWithProtocol(ProtocolIPv4) failed: %v", err)
				}
				ipList, _ = ip4t.List("nat", "POSTROUTING")
		}
		for _, _r := range vm_host_record {
			// fmt.Println("Route", _r)
			if *r_context == "google_route" {
				get_or_create_route(service, projectid, _r)
			} else {
				if *vmIptables == _r.Vm {
					fmt.Println("Iptables started.")
					if lastcname == "" || (lastcname != generateChainName(_r.route_chain)) {
					cname = generateChain(_r.route_chain)
					lastcname = cname
				}

				fmt.Println(_r,ip4t,ipList)
//				Initilisation
				if len(ipList) <= 1 {
					iptablesfirstLine()
					iptablesLastLine( len(ipList) )
				}
				iptablesUpdate(_r, cname)
				appendToPostrouting(cname)
				}
			}

		}
		time.Sleep(10000 * time.Millisecond)
	}
	

	
//	http.Handle("/metrics", promhttp.Handler())
//	log.Fatal(http.ListenAndServe(*addr, nil))

	// fmt.Println("Full data", vm_host_record)

	// // Show the current images that are available.
	// res, err := service.Images.List(projectId).Do()
	// log.Printf("Got compute.Images.List, err: %#v, %v", res, err)

}

func verify(){
	list, err := ip4t.List("nat", "POSTROUTING")
	if err != nil{
		fmt.Printf("ERROR while getting the list: %v",err)
	}

	fmt.Printf("List is %v:  ",list)
	listSize := len(list)
	for index, v := range list {
		if index == 1 {
			if v != "-A POSTROUTING -j LOG"{
				fmt.Printf("POSTROUTING chain's first line should be -A POSTROUTING -j LOG, but it is:%v. Fixing",v )
				er := iptablesfirstLine()
				if er != nil{
					fmt.Printf("POSTROUTING chain's first insert error -A POSTROUTING -j LOG, but it is:%v. issue",er )
				}
			}
		} else if index == (listSize -1 ){
			if v != "-A POSTROUTING -o eth0 -j MASQUERADE"{
				fmt.Printf("POSTROUTING chain's last line should be -A POSTROUTING -o eth0 -j MASQUERADE, but it is:%v. Fixing",v )
				er := iptablesLastLine(index+1)
				if er != nil{
					fmt.Printf("POSTROUTING chain's last line should be -A POSTROUTING -o eth0 -j MASQUERADE, but it is:%v. issue",er )
				}
			}
		}
		/*else if  index != (listSize -1 ) {
			if v == "-A POSTROUTING -o eth0 -j MASQUERADE"{
				fmt.Printf("POSTROUTING chain's %i line should not be -A POSTROUTING -o eth0 -j MASQUERADE, but it is:%v. Fixing",v )
				er := iptablesLastLine(index+1)
				if er != nil{
					fmt.Printf("POSTROUTING chain's last line should be -A POSTROUTING -o eth0 -j MASQUERADE, but it is:%v. issue",er )
				}
			}
		}*/
	}
}

func http_request() string {
	timeout := time.Duration(5 * time.Second)
	client := &http.Client{
		Timeout: timeout,
	}
	body := ""
	counter := true
	for loop_variable := 0; counter && loop_variable < 5; loop_variable = loop_variable + 1 {
		req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/name", nil)
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := client.Do(req)
		if err != nil {
			b, _ := ioutil.ReadAll(resp.Body)
			fmt.Println(string(b))
			// log.Fatal(string(b))
			// log.Fatal("There seems to be some problem with goot meta data")
		}
		if resp.StatusCode == 503 {
			fmt.Println("Got 503 sleeping for 100 milisec.")
			time.Sleep(100 * time.Millisecond)
		}
		b,_ := ioutil.ReadAll(resp.Body)
		body = string(b)
		counter = false
	}
	return body

}
