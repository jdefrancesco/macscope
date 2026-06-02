package logquery

const (
	// Prdicate to find TCC related log messages. TCC is the Trust
	TCCPredicate = `process == "tccd" OR subsystem CONTAINS[c] "TCC" OR eventMessage CONTAINS[c] "kTCC" OR eventMessage CONTAINS[c] "deny"`
	// Attach predicate
	AttachPredicate = `eventMessage CONTAINS[c] "task_for_pid" OR eventMessage CONTAINS[c] "debug" OR process == "amfid" OR process == "tccd" OR process == "taskgated" OR process == "syspolicyd"`
	// EndpointSecurityPredicate will display any activity if an event occurs or it is being interfaced with via some other means.
	EndpointSecurityPredicate = `eventMessage CONTAINS[c] "EndpointSecurity" OR eventMessage CONTAINS[c] "/dev/es" OR eventMessage CONTAINS[c] "com.apple.developer.endpoint-security.client"`
	// VPN predicate to check if VPN is active or causing issues like dropped connections.
	VPNPredicate = `subsystem CONTAINS[c] "vpn" OR process CONTAINS[c] "vpn" OR eventMessage CONTAINS[c] "utun" OR eventMessage CONTAINS[c] "IPSec" OR eventMessage CONTAINS[c] "IKE" OR eventMessage CONTAINS[c] "disconnect"`
	// Panic predicate. You see this, nothing good is coming your way fam.
	PanicPredicate = `eventMessage CONTAINS[c] "DumpPanic" OR eventMessage CONTAINS[c] "panic" OR eventMessage CONTAINS[c] "watchdog" OR eventMessage CONTAINS[c] "IOKit Boot"`
)

func Predicate(name string) (string, bool) {
	switch name {
	case "tcc":
		return TCCPredicate, true
	case "attach":
		return AttachPredicate, true
	case "es":
		return EndpointSecurityPredicate, true
	case "vpn":
		return VPNPredicate, true
	case "panic":
		return PanicPredicate, true
	default:
		return "", false
	}
}
