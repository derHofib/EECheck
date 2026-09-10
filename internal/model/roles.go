package model

import "strings"

// RoleFromEntityType infers the coarse device role from a SPINE entity type
// name (model.EntityTypeType stringified, e.g. "EVSE", "Inverter"). Kept as
// a plain string parameter so internal/model does not depend on spine-go.
//
// Mapping verified against the entity types actually referenced by the
// eg/lpc and eg/lpp use cases in eebus-go (see docs/05-recherche-antworten.md):
// LPC targets EVSE/HeatPumpAppliance/Compressor/SmartEnergyAppliance
// (consumption direction), LPP targets EVSE/Inverter/SmartEnergyAppliance
// (production direction). A device can legitimately support both (e.g. a
// bidirectional wallbox or a battery inverter), so this is only a default
// hint for the GUI icon/label, never a hard restriction on which use cases
// are attempted against it.
func RoleFromEntityType(entityType string) DeviceRole {
	switch strings.ToLower(entityType) {
	case "inverter":
		return RoleStorage // could be pure PV or PV+battery; LPP always applies, LPC may also apply
	case "evse":
		return RoleConsumer // wallboxes are typically LPC targets; bidirectional (V2G) ones also expose LPP
	case "heatpumpappliance", "compressor", "smartenergyappliance":
		return RoleConsumer
	case "submeterelectricity":
		return RoleUnknown
	default:
		return RoleUnknown
	}
}

// RoleFromMdnsType is a coarser, pre-pairing heuristic based on the mDNS
// TXT "type" field (e.g. "EVSE", "Charger", "Inverter") reported before any
// SPINE detail discovery has happened. Used only for the discovery list
// icon; RoleFromEntityType (post-pairing, SPINE-based) is authoritative.
func RoleFromMdnsType(mdnsType string) DeviceRole {
	t := strings.ToLower(mdnsType)
	switch {
	case strings.Contains(t, "evse") || strings.Contains(t, "charg") || strings.Contains(t, "wallbox"):
		return RoleConsumer
	case strings.Contains(t, "inverter") || strings.Contains(t, "pv") || strings.Contains(t, "battery") || strings.Contains(t, "storage"):
		return RoleStorage
	case strings.Contains(t, "heatpump") || strings.Contains(t, "compressor"):
		return RoleConsumer
	default:
		return RoleUnknown
	}
}
