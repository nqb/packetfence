package pfipset

import (
	"context"
	"crypto/tls"
	"database/sql"
	"io"
	"net"
	"net/http"
	"regexp"

	ipset "github.com/digineo/go-ipset"
	"github.com/inverse-inc/packetfence/go/log"
	"github.com/inverse-inc/packetfence/go/pfconfigdriver"
)

var body io.Reader

type pfIPSET struct {
	Network map[*net.IPNet]string
	ListALL []ipset.IPSet
	jobs    chan job
}

func pfIPSETFromContext(ctx context.Context) *pfIPSET {
	return ctx.Value("IPSET").(*pfIPSET)
}

func (IPSET *pfIPSET) AddToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "IPSET", IPSET)
}

// Detect the vip on each interfaces
func getClusterMembersIps(ctx context.Context) []net.IP {

	var keyConfCluster pfconfigdriver.PfconfigKeys
	keyConfCluster.PfconfigNS = "resource::cluster_hosts_ip"

	pfconfigdriver.FetchDecodeSocket(ctx, &keyConfCluster)
	var members []net.IP
	for _, key := range keyConfCluster.Keys {
		var ConfNet pfconfigdriver.PfClusterIp
		ConfNet.PfconfigHashNS = key

		pfconfigdriver.FetchDecodeSocket(ctx, &ConfNet)

		IP := net.ParseIP(ConfNet.Ip)
		var present bool

		ifaces, _ := net.Interfaces()
		for _, netInterface := range ifaces {
			addrs, _ := netInterface.Addrs()
			for _, UnicastAddr := range addrs {
				IPE, _, _ := net.ParseCIDR(UnicastAddr.String())
				if IP.Equal(IPE) {
					present = true
				}
			}
		}
		if present == false {
			members = append(members, IP)
		}
	}
	return members
}

func updateClusterL2(ctx context.Context, Ip string, Mac string, Network string, Type string, Catid string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/mark_layer2/"+Network+"/"+Type+"/"+Catid+"/"+Ip+"/"+Mac+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func updateClusterL3(ctx context.Context, Ip string, Network string, Type string, Catid string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/mark_layer3/"+Network+"/"+Type+"/"+Catid+"/"+Ip+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func updateClusterUnmarkMac(ctx context.Context, Mac string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/unmark_mac/"+Mac+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func updateClusterUnmarkIp(ctx context.Context, Ip string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {

		err := post(ctx, "https://"+member.String()+":22223/ipset/unmark_ip/"+Ip+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func updateClusterMarkIpL3(ctx context.Context, Ip string, Network string, Catid string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/mark_ip_layer3/"+Network+"/"+Catid+"/"+Ip+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}
func updateClusterMarkIpL2(ctx context.Context, Ip string, Network string, Catid string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/mark_ip_layer2/"+Network+"/"+Catid+"/"+Ip+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func updateClusterPassthrough(ctx context.Context, Ip string, Port string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/passthrough/"+Ip+"/"+Port+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func updateClusterPassthroughIsol(ctx context.Context, Ip string, Port string) {
	logger := log.LoggerWContext(ctx)

	for _, member := range getClusterMembersIps(ctx) {
		err := post(ctx, "https://"+member.String()+":22223/ipset/passthrough_isolation/"+Ip+"/"+Port+"/1", body)
		if err != nil {
			logger.Error("Not able to contact " + member.String() + err.Error())
		} else {
			logger.Info("Updated " + member.String())
		}
	}
}

func (IPSET *pfIPSET) mac2ip(ctx context.Context, Mac string, Set ipset.IPSet) []string {
	r := "((?:[0-9]{1,3}.){3}(?:[0-9]{1,3}))," + Mac

	rgx := regexp.MustCompile(r)

	var Ips []string
	for _, u := range Set.Members {

		if rgx.Match([]byte(u.Elem)) {
			result := rgx.FindStringSubmatch(u.Elem)

			Ips = append(Ips, result[1])
		}
	}
	return Ips
}

func post(ctx context.Context, url string, body io.Reader) error {
	req, err := http.NewRequest("POST", url, body)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Transport: tr}
	_, err = cli.Do(req)
	return err
}

// initIPSet fetch the database to remove already assigned ip addresses
func (IPSET *pfIPSET) initIPSet(ctx context.Context, db *sql.DB) {
	logger := log.LoggerWContext(ctx)

	IPSET.ListALL, _ = ipset.ListAll()
	rows, err := db.Query("select distinct n.mac, i.ip, n.category_id from node as n left join locationlog as l on n.mac=l.mac left join ip4log as i on n.mac=i.mac where l.connection_type = \"inline\" and n.status=\"reg\" and n.mac=i.mac and i.end_time > NOW()")
	if err != nil {
		// Log here
		logger.Error(err.Error())
		return
	}
	defer rows.Close()
	var (
		ipstr string
		mac   string
		catID string
	)
	for rows.Next() {
		err := rows.Scan(&mac, &ipstr, &catID)
		if err != nil {
			// Log here
			logger.Error(err.Error())
			return
		}
		for k, v := range IPSET.Network {
			if k.Contains(net.ParseIP(ipstr)) {
				if v == "inlinel2" {
					IPSET.IPSEThandleLayer2(ctx, ipstr, mac, k.IP.String(), "Reg", catID)
					IPSET.IPSEThandleMarkIpL2(ctx, ipstr, k.IP.String(), catID)
				}
				if v == "inlinel3" {
					IPSET.IPSEThandleLayer3(ctx, ipstr, k.IP.String(), "Reg", catID)
					IPSET.IPSEThandleMarkIpL3(ctx, ipstr, k.IP.String(), catID)
				}
				break
			}
		}
	}
}

// detectType of each network
func (IPSET *pfIPSET) detectType(ctx context.Context) error {
	IPSET.ListALL, _ = ipset.ListAll()
	var NetIndex net.IPNet
	IPSET.Network = make(map[*net.IPNet]string)

	var interfaces pfconfigdriver.ListenInts
	pfconfigdriver.FetchDecodeSocket(ctx, &interfaces)

	var keyConfNet pfconfigdriver.PfconfigKeys
	keyConfNet.PfconfigNS = "config::Network"
	pfconfigdriver.FetchDecodeSocket(ctx, &keyConfNet)

	var keyConfCluster pfconfigdriver.NetInterface
	keyConfCluster.PfconfigNS = "config::Pf(CLUSTER)"

	for _, v := range interfaces.Element {

		keyConfCluster.PfconfigHashNS = "interface " + v
		pfconfigdriver.FetchDecodeSocket(ctx, &keyConfCluster)
		// Nothing in keyConfCluster.Ip so we are not in cluster mode

		eth, _ := net.InterfaceByName(v)
		adresses, _ := eth.Addrs()
		for _, adresse := range adresses {

			var NetIP *net.IPNet
			var IP net.IP
			IP, NetIP, _ = net.ParseCIDR(adresse.String())
			if IP.To4() == nil {
				continue
			}
			a, b := NetIP.Mask.Size()
			if a == b {
				continue
			}

			for _, key := range keyConfNet.Keys {
				var ConfNet pfconfigdriver.NetworkConf
				ConfNet.PfconfigHashNS = key
				pfconfigdriver.FetchDecodeSocket(ctx, &ConfNet)
				if (NetIP.Contains(net.ParseIP(ConfNet.DhcpStart)) && NetIP.Contains(net.ParseIP(ConfNet.DhcpEnd))) || NetIP.Contains(net.ParseIP(ConfNet.NextHop)) {
					NetIndex.Mask = net.IPMask(net.ParseIP(ConfNet.Netmask))
					NetIndex.IP = net.ParseIP(key)
					Index := NetIndex
					IPSET.Network[&Index] = ConfNet.Type
				}
			}
		}
	}
	return nil
}
