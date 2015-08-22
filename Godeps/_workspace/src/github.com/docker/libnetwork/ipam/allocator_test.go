package ipam

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/docker/libnetwork/bitseq"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	_ "github.com/docker/libnetwork/netutils"
)

var ds datastore.DataStore

// enable w/ upper case
func testMain(m *testing.M) {
	var err error
	ds, err = datastore.NewDataStore(&config.DatastoreCfg{Embedded: false, Client: config.DatastoreClientCfg{Provider: "consul", Address: "127.0.0.1:8500"}})
	if err != nil {
		fmt.Println(err)
	}

	os.Exit(m.Run())
}

func getAllocator(t *testing.T, subnet *net.IPNet) *Allocator {
	a, err := NewAllocator(ds)
	if err != nil {
		t.Fatal(err)
	}
	a.AddSubnet("default", &SubnetInfo{Subnet: subnet})
	return a
}

func TestInt2IP2IntConversion(t *testing.T) {
	for i := uint32(0); i < 256*256*256; i++ {
		var array [4]byte // new array at each cycle
		addIntToIP(array[:], i)
		j := ipToUint32(array[:])
		if j != i {
			t.Fatalf("Failed to convert ordinal %d to IP % x and back to ordinal. Got %d", i, array, j)
		}
	}
}

func TestGetAddressVersion(t *testing.T) {
	if v4 != getAddressVersion(net.ParseIP("172.28.30.112")) {
		t.Fatalf("Failed to detect IPv4 version")
	}
	if v4 != getAddressVersion(net.ParseIP("0.0.0.1")) {
		t.Fatalf("Failed to detect IPv4 version")
	}
	if v6 != getAddressVersion(net.ParseIP("ff01::1")) {
		t.Fatalf("Failed to detect IPv6 version")
	}
	if v6 != getAddressVersion(net.ParseIP("2001:56::76:51")) {
		t.Fatalf("Failed to detect IPv6 version")
	}
}

func TestKeyString(t *testing.T) {

	k := &subnetKey{addressSpace: "default", subnet: "172.27.0.0/16"}
	expected := "default/172.27.0.0/16"
	if expected != k.String() {
		t.Fatalf("Unexpected key string: %s", k.String())
	}

	k2 := &subnetKey{}
	err := k2.FromString(expected)
	if err != nil {
		t.Fatal(err)
	}
	if k2.addressSpace != k.addressSpace || k2.subnet != k.subnet {
		t.Fatalf("subnetKey.FromString() failed. Expected %v. Got %v", k, k2)
	}

	expected = fmt.Sprintf("%s/%s", expected, "172.27.3.0/24")
	k.childSubnet = "172.27.3.0/24"
	if expected != k.String() {
		t.Fatalf("Unexpected key string: %s", k.String())
	}

	err = k2.FromString(expected)
	if err != nil {
		t.Fatal(err)
	}
	if k2.addressSpace != k.addressSpace || k2.subnet != k.subnet {
		t.Fatalf("subnetKey.FromString() failed. Expected %v. Got %v", k, k2)
	}
}

func TestAddSubnets(t *testing.T) {
	a, err := NewAllocator(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, sub0, _ := net.ParseCIDR("10.0.0.0/8")
	err = a.AddSubnet("default", &SubnetInfo{Subnet: sub0})
	if err != nil {
		t.Fatalf("Unexpected failure in adding subent")
	}

	err = a.AddSubnet("abc", &SubnetInfo{Subnet: sub0})
	if err != nil {
		t.Fatalf("Unexpected failure in adding overlapping subents to different address spaces")
	}

	err = a.AddSubnet("abc", &SubnetInfo{Subnet: sub0})
	if err == nil {
		t.Fatalf("Failed to detect overlapping subnets: %s and %s", sub0, sub0)
	}

	_, sub1, _ := net.ParseCIDR("10.20.2.0/24")
	err = a.AddSubnet("default", &SubnetInfo{Subnet: sub1})
	if err == nil {
		t.Fatalf("Failed to detect overlapping subnets: %s and %s", sub0, sub1)
	}

	_, sub2, _ := net.ParseCIDR("10.128.0.0/9")
	err = a.AddSubnet("default", &SubnetInfo{Subnet: sub2})
	if err == nil {
		t.Fatalf("Failed to detect overlapping subnets: %s and %s", sub1, sub2)
	}

	_, sub6, err := net.ParseCIDR("1003:1:2:3:4:5:6::/112")
	if err != nil {
		t.Fatalf("Wrong input, Can't proceed: %s", err.Error())
	}
	err = a.AddSubnet("default", &SubnetInfo{Subnet: sub6})
	if err != nil {
		t.Fatalf("Failed to add v6 subnet: %s", err.Error())
	}

	_, sub6, err = net.ParseCIDR("1003:1:2:3::/64")
	if err != nil {
		t.Fatalf("Wrong input, Can't proceed: %s", err.Error())
	}
	err = a.AddSubnet("default", &SubnetInfo{Subnet: sub6})
	if err == nil {
		t.Fatalf("Failed to detect overlapping v6 subnet")
	}
}

func TestAdjustAndCheckSubnet(t *testing.T) {
	_, sub6, _ := net.ParseCIDR("1003:1:2:300::/63")
	_, err := adjustAndCheckSubnetSize(sub6)
	if err == nil {
		t.Fatalf("Failed detect too big v6 subnet")
	}

	_, sub, _ := net.ParseCIDR("192.0.0.0/7")
	_, err = adjustAndCheckSubnetSize(sub)
	if err == nil {
		t.Fatalf("Failed detect too big v4 subnet")
	}

	subnet := "1004:1:2:6::/64"
	_, sub6, _ = net.ParseCIDR(subnet)
	subnetToSplit, err := adjustAndCheckSubnetSize(sub6)
	if err != nil {
		t.Fatalf("Unexpected error returned by adjustAndCheckSubnetSize()")
	}
	ones, _ := subnetToSplit.Mask.Size()
	if ones < minNetSizeV6Eff {
		t.Fatalf("Wrong effective network size for %s. Expected: %d. Got: %d", subnet, minNetSizeV6Eff, ones)
	}
}

func TestRemoveSubnet(t *testing.T) {
	a, err := NewAllocator(nil)
	if err != nil {
		t.Fatal(err)
	}

	input := []struct {
		addrSpace AddressSpace
		subnet    string
	}{
		{"default", "192.168.0.0/16"},
		{"default", "172.17.0.0/16"},
		{"default", "10.0.0.0/8"},
		{"default", "2002:1:2:3:4:5:ffff::/112"},
		{"splane", "172.17.0.0/16"},
		{"splane", "10.0.0.0/8"},
		{"splane", "2002:1:2:3:4:5:6::/112"},
		{"splane", "2002:1:2:3:4:5:ffff::/112"},
	}

	for _, i := range input {
		_, sub, err := net.ParseCIDR(i.subnet)
		if err != nil {
			t.Fatalf("Wrong input, Can't proceed: %s", err.Error())
		}
		err = a.AddSubnet(i.addrSpace, &SubnetInfo{Subnet: sub})
		if err != nil {
			t.Fatalf("Failed to apply input. Can't proceed: %s", err.Error())
		}
	}

	_, sub, _ := net.ParseCIDR("172.17.0.0/16")
	a.RemoveSubnet("default", sub)
	if len(a.subnets) != 7 {
		t.Fatalf("Failed to remove subnet info")
	}
	list := a.getSubnetList("default", v4)
	if len(list) != 257 {
		t.Fatalf("Failed to effectively remove subnet address space")
	}

	_, sub, _ = net.ParseCIDR("2002:1:2:3:4:5:ffff::/112")
	a.RemoveSubnet("default", sub)
	if len(a.subnets) != 6 {
		t.Fatalf("Failed to remove subnet info")
	}
	list = a.getSubnetList("default", v6)
	if len(list) != 0 {
		t.Fatalf("Failed to effectively remove subnet address space")
	}

	_, sub, _ = net.ParseCIDR("2002:1:2:3:4:5:6::/112")
	a.RemoveSubnet("splane", sub)
	if len(a.subnets) != 5 {
		t.Fatalf("Failed to remove subnet info")
	}
	list = a.getSubnetList("splane", v6)
	if len(list) != 1 {
		t.Fatalf("Failed to effectively remove subnet address space")
	}
}

func TestGetInternalSubnets(t *testing.T) {
	// This function tests the splitting of a parent subnet in small host subnets.
	// The splitting is controlled by the max host size, which is the first parameter
	// passed to the function. It basically says if the parent subnet host size is
	// greater than the max host size, split the parent subnet into N internal small
	// subnets with host size = max host size to cover the same address space.

	input := []struct {
		internalHostSize int
		parentSubnet     string
		firstIntSubnet   string
		lastIntSubnet    string
	}{
		// Test 8 bits prefix network
		{24, "10.0.0.0/8", "10.0.0.0/8", "10.0.0.0/8"},
		{16, "10.0.0.0/8", "10.0.0.0/16", "10.255.0.0/16"},
		{8, "10.0.0.0/8", "10.0.0.0/24", "10.255.255.0/24"},
		// Test 16 bits prefix network
		{16, "192.168.0.0/16", "192.168.0.0/16", "192.168.0.0/16"},
		{8, "192.168.0.0/16", "192.168.0.0/24", "192.168.255.0/24"},
		// Test 24 bits prefix network
		{16, "192.168.57.0/24", "192.168.57.0/24", "192.168.57.0/24"},
		{8, "192.168.57.0/24", "192.168.57.0/24", "192.168.57.0/24"},
		// Test non byte multiple host size
		{24, "10.0.0.0/8", "10.0.0.0/8", "10.0.0.0/8"},
		{20, "10.0.0.0/12", "10.0.0.0/12", "10.0.0.0/12"},
		{20, "10.128.0.0/12", "10.128.0.0/12", "10.128.0.0/12"},
		{12, "10.16.0.0/16", "10.16.0.0/20", "10.16.240.0/20"},
		{13, "10.0.0.0/8", "10.0.0.0/19", "10.255.224.0/19"},
		{15, "10.0.0.0/8", "10.0.0.0/17", "10.255.128.0/17"},
		// Test v6 network
		{16, "2002:1:2:3:4:5:6000::/110", "2002:1:2:3:4:5:6000:0/112", "2002:1:2:3:4:5:6003:0/112"},
		{16, "2002:1:2:3:4:5:ff00::/104", "2002:1:2:3:4:5:ff00:0/112", "2002:1:2:3:4:5:ffff:0/112"},
		{12, "2002:1:2:3:4:5:ffff::/112", "2002:1:2:3:4:5:ffff:0/116", "2002:1:2:3:4:5:ffff:f000/116"},
		{11, "2002:1:2:3:4:5:ffff::/112", "2002:1:2:3:4:5:ffff:0/117", "2002:1:2:3:4:5:ffff:f800/117"},
	}

	for _, d := range input {
		assertInternalSubnet(t, d.internalHostSize, d.parentSubnet, d.firstIntSubnet, d.lastIntSubnet)
	}
}

func TestGetSameAddress(t *testing.T) {
	a, err := NewAllocator(nil)
	if err != nil {
		t.Fatal(err)
	}

	addSpace := AddressSpace("giallo")
	_, subnet, _ := net.ParseCIDR("192.168.100.0/24")
	if err := a.AddSubnet(addSpace, &SubnetInfo{Subnet: subnet}); err != nil {
		t.Fatal(err)
	}

	ip := net.ParseIP("192.168.100.250")
	req := &AddressRequest{Subnet: *subnet, Address: ip}

	_, err = a.Request(addSpace, req)
	if err != nil {
		t.Fatal(err)
	}

	_, err = a.Request(addSpace, req)
	if err == nil {
		t.Fatal(err)
	}
}

func TestGetAddress(t *testing.T) {
	input := []string{
		/*"10.0.0.0/8", "10.0.0.0/9", */ "10.0.0.0/10", "10.0.0.0/11", "10.0.0.0/12", "10.0.0.0/13", "10.0.0.0/14",
		"10.0.0.0/15", "10.0.0.0/16", "10.0.0.0/17", "10.0.0.0/18", "10.0.0.0/19", "10.0.0.0/20", "10.0.0.0/21",
		"10.0.0.0/22", "10.0.0.0/23", "10.0.0.0/24", "10.0.0.0/25", "10.0.0.0/26", "10.0.0.0/27", "10.0.0.0/28",
		"10.0.0.0/29", "10.0.0.0/30", "10.0.0.0/31"}

	for _, subnet := range input {
		assertGetAddress(t, subnet)
	}
}

func TestGetSubnetList(t *testing.T) {
	a, err := NewAllocator(nil)
	if err != nil {
		t.Fatal(err)
	}
	input := []struct {
		addrSpace AddressSpace
		subnet    string
	}{
		{"default", "192.168.0.0/16"},
		{"default", "172.17.0.0/16"},
		{"default", "10.0.0.0/8"},
		{"default", "2002:1:2:3:4:5:6::/112"},
		{"default", "2002:1:2:3:4:5:ffff::/112"},
		{"splane", "172.17.0.0/16"},
		{"splane", "10.0.0.0/8"},
		{"splane", "2002:1:2:3:4:5:ff00::/104"},
	}

	for _, i := range input {
		_, sub, err := net.ParseCIDR(i.subnet)
		if err != nil {
			t.Fatalf("Wrong input, Can't proceed: %s", err.Error())
		}
		err = a.AddSubnet(i.addrSpace, &SubnetInfo{Subnet: sub})
		if err != nil {
			t.Fatalf("Failed to apply input. Can't proceed: %s", err.Error())
		}
	}

	list := a.getSubnetList("default", v4)
	if len(list) != 258 {
		t.Fatalf("Incorrect number of internal subnets for ipv4 version. Expected 258. Got %d.", len(list))
	}
	list = a.getSubnetList("splane", v4)
	if len(list) != 257 {
		t.Fatalf("Incorrect number of internal subnets for ipv4 version. Expected 257. Got %d.", len(list))
	}

	list = a.getSubnetList("default", v6)
	if len(list) != 2 {
		t.Fatalf("Incorrect number of internal subnets for ipv6 version. Expected 2. Got %d.", len(list))
	}
	list = a.getSubnetList("splane", v6)
	if len(list) != 256 {
		t.Fatalf("Incorrect number of internal subnets for ipv6 version. Expected 256. Got %d.", len(list))
	}

}

func TestRequestSyntaxCheck(t *testing.T) {
	var (
		subnet   = "192.168.0.0/16"
		addSpace = AddressSpace("green")
	)

	a, err := NewAllocator(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add subnet and create base request
	_, sub, _ := net.ParseCIDR(subnet)
	a.AddSubnet(addSpace, &SubnetInfo{Subnet: sub})
	req := &AddressRequest{Subnet: *sub}

	// Empty address space request
	_, err = a.Request("", req)
	if err == nil {
		t.Fatalf("Failed to detect wrong request: empty address space")
	}

	// Preferred address from different subnet in request
	req.Address = net.ParseIP("172.17.0.23")
	_, err = a.Request(addSpace, req)
	if err == nil {
		t.Fatalf("Failed to detect wrong request: preferred IP from different subnet")
	}

	// Preferred address specified and nil subnet
	req = &AddressRequest{Address: net.ParseIP("172.17.0.23")}
	_, err = a.Request(addSpace, req)
	if err == nil {
		t.Fatalf("Failed to detect wrong request: subnet not specified but preferred address specified")
	}
}

func TestRequest(t *testing.T) {
	// Request N addresses from different size subnets, verifying last request
	// returns expected address. Internal subnet host size is Allocator's default, 16
	input := []struct {
		subnet string
		numReq int
		lastIP string
	}{
		{"192.168.59.0/24", 254, "192.168.59.254"},
		{"192.168.240.0/20", 255, "192.168.240.255"},
		{"192.168.0.0/16", 255, "192.168.0.255"},
		{"192.168.0.0/16", 256, "192.168.1.0"},
		{"10.16.0.0/16", 255, "10.16.0.255"},
		{"10.128.0.0/12", 255, "10.128.0.255"},
		{"10.0.0.0/8", 256, "10.0.1.0"},

		{"192.168.128.0/18", 4*256 - 1, "192.168.131.255"},
		{"192.168.240.0/20", 16*256 - 2, "192.168.255.254"},

		{"192.168.0.0/16", 256*256 - 2, "192.168.255.254"},
		{"10.0.0.0/8", 2 * 256, "10.0.2.0"},
		{"10.0.0.0/8", 5 * 256, "10.0.5.0"},
		//{"10.0.0.0/8", 100 * 256 * 254, "10.99.255.254"},
	}

	for _, d := range input {
		assertNRequests(t, d.subnet, d.numReq, d.lastIP)
	}
}

func TestRelease(t *testing.T) {
	var (
		err    error
		req    *AddressRequest
		subnet = "192.168.0.0/16"
	)

	_, sub, _ := net.ParseCIDR(subnet)
	a := getAllocator(t, sub)
	req = &AddressRequest{Subnet: *sub}
	bm := a.addresses[subnetKey{"default", subnet, subnet}]

	// Allocate all addresses
	for err != ErrNoAvailableIPs {
		_, err = a.Request("default", req)
	}

	toRelease := []struct {
		address string
	}{
		{"192.168.0.1"},
		{"192.168.0.2"},
		{"192.168.0.3"},
		{"192.168.0.4"},
		{"192.168.0.5"},
		{"192.168.0.6"},
		{"192.168.0.7"},
		{"192.168.0.8"},
		{"192.168.0.9"},
		{"192.168.0.10"},
		{"192.168.0.30"},
		{"192.168.0.31"},
		{"192.168.1.32"},

		{"192.168.0.254"},
		{"192.168.1.1"},
		{"192.168.1.2"},

		{"192.168.1.3"},

		{"192.168.255.253"},
		{"192.168.255.254"},
	}

	// One by one, relase the address and request again. We should get the same IP
	req = &AddressRequest{Subnet: *sub}
	for i, inp := range toRelease {
		address := net.ParseIP(inp.address)
		a.Release("default", address)
		if bm.Unselected() != 1 {
			t.Fatalf("Failed to update free address count after release. Expected %d, Found: %d", i+1, bm.Unselected())
		}

		rsp, err := a.Request("default", req)
		if err != nil {
			t.Fatalf("Failed to obtain the address: %s", err.Error())
		}
		if !address.Equal(rsp.Address) {
			t.Fatalf("Failed to obtain the same address. Expected: %s, Got: %s", address, rsp.Address)
		}
	}
}

func assertInternalSubnet(t *testing.T, hostSize int, bigSubnet, firstSmall, lastSmall string) {
	_, subnet, _ := net.ParseCIDR(bigSubnet)
	list, _ := getInternalSubnets(subnet, hostSize)
	count := 1
	ones, bits := subnet.Mask.Size()
	diff := bits - ones - int(hostSize)
	if diff > 0 {
		count <<= uint(diff)
	}

	if len(list) != count {
		t.Fatalf("Wrong small subnets number. Expected: %d, Got: %d", count, len(list))
	}
	if firstSmall != list[0].String() {
		t.Fatalf("Wrong first small subent. Expected: %v, Got: %v", firstSmall, list[0])
	}
	if lastSmall != list[count-1].String() {
		t.Fatalf("Wrong last small subent. Expected: %v, Got: %v", lastSmall, list[count-1])
	}
}

func assertGetAddress(t *testing.T, subnet string) {
	var (
		err       error
		printTime = false
		a         = &Allocator{}
	)

	_, sub, _ := net.ParseCIDR(subnet)
	ones, bits := sub.Mask.Size()
	zeroes := bits - ones
	numAddresses := 1 << uint(zeroes)

	bm, err := bitseq.NewHandle("ipam_test", nil, "default/192.168.0.0/24", uint32(numAddresses))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	run := 0
	for err != ErrNoAvailableIPs {
		_, err = a.getAddress(sub, bm, nil, v4)
		run++
	}
	if printTime {
		fmt.Printf("\nTaken %v, to allocate all addresses on %s. (nemAddresses: %d. Runs: %d)", time.Since(start), subnet, numAddresses, run)
	}
	if bm.Unselected() != 0 {
		t.Fatalf("Unexpected free count after reserving all addresses: %d", bm.Unselected())
	}
	/*
		if bm.Head.Block != expectedMax || bm.Head.Count != numBlocks {
			t.Fatalf("Failed to effectively reserve all addresses on %s. Expected (0x%x, %d) as first sequence. Found (0x%x,%d)",
				subnet, expectedMax, numBlocks, bm.Head.Block, bm.Head.Count)
		}
	*/
}

func assertNRequests(t *testing.T, subnet string, numReq int, lastExpectedIP string) {
	var (
		err       error
		req       *AddressRequest
		rsp       *AddressResponse
		printTime = false
	)

	_, sub, _ := net.ParseCIDR(subnet)
	lastIP := net.ParseIP(lastExpectedIP)

	a := getAllocator(t, sub)
	req = &AddressRequest{Subnet: *sub}

	i := 0
	start := time.Now()
	for ; i < numReq; i++ {
		rsp, err = a.Request("default", req)
	}
	if printTime {
		fmt.Printf("\nTaken %v, to allocate %d addresses on %s\n", time.Since(start), numReq, subnet)
	}

	if !lastIP.Equal(rsp.Address) {
		t.Fatalf("Wrong last IP. Expected %s. Got: %s (err: %v, ind: %d)", lastExpectedIP, rsp.Address.String(), err, i)
	}
}

func benchmarkRequest(subnet *net.IPNet) {
	var err error

	a, _ := NewAllocator(nil)
	a.internalHostSize = 20
	a.AddSubnet("default", &SubnetInfo{Subnet: subnet})

	req := &AddressRequest{Subnet: *subnet}
	for err != ErrNoAvailableIPs {
		_, err = a.Request("default", req)

	}
}

func benchMarkRequest(subnet *net.IPNet, b *testing.B) {
	for n := 0; n < b.N; n++ {
		benchmarkRequest(subnet)
	}
}

func BenchmarkRequest_24(b *testing.B) {
	benchmarkRequest(&net.IPNet{IP: []byte{10, 0, 0, 0}, Mask: []byte{255, 255, 255, 0}})
}

func BenchmarkRequest_16(b *testing.B) {
	benchmarkRequest(&net.IPNet{IP: []byte{10, 0, 0, 0}, Mask: []byte{255, 255, 0, 0}})
}

func BenchmarkRequest_8(b *testing.B) {
	benchmarkRequest(&net.IPNet{IP: []byte{10, 0, 0, 0}, Mask: []byte{255, 0xfc, 0, 0}})
}
