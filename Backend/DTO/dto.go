package DTO

import (
	"net"
	"time"
)

// NetworkInfo: Tarayıcının çalışması için gereken yerel ağ bilgileri
type NetworkInfo struct {
	InterfaceName string     // "eno1" (ARP tablosunu filtrelemek için şart) / NIC karti adi
	LocalIP       net.IP     // "192.168.1.11" (Kendi IP'miz)
	Subnet        *net.IPNet // 192.168.1.0/24 (Taranacak mahalle ve maske)
	TotalHosts    int        // 254 (Döngü sınırı)
}

type DeviceOutput struct {
	Ip         net.IP
	Mac        net.HardwareAddr
	LatencyMs  time.Duration
	Vendor     string
	OpenPorts  []int
	DeviceType string
}
