package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/nodes"
	corev1 "k8s.io/api/core/v1"
	"github.com/openshift/cluster-node-tuning-operator/test/e2e/performanceprofile/functests/utils/mylog"
	
)

// CpuArchitecture  struct to represent CPU Details
type CpuArchitecture struct {
	Lscpu []cpuField `json:"lscpu"`
}
type cpuField struct {
	Field string `json:"field"`
	Data  string `json:"data"`
}

const (
	IntelVendorID = "GenuineIntel"
	AMDVendorID   = "AuthenticAMD"
)

// lscpuPraser parses lscpu output and returns its fields in struct
func lscpuPraser(ctx context.Context, node *corev1.Node, logger *mylog.TestLogger) (CpuArchitecture, error) {
	cmd := []string{"lscpu", "-J"}
	var cpuinfo CpuArchitecture
	out, err := nodes.ExecCommand(ctx, node, cmd, logger)
	if err != nil {
		return cpuinfo, fmt.Errorf("error executing lscpu command: %v", err)
	}
	err = json.Unmarshal(out, &cpuinfo)
	if err != nil {
		return cpuinfo, fmt.Errorf("error unmarshalling cpu info: %v", err)
	}
	return cpuinfo, nil
}

// CPUArchitecture returns CPU Architecture from lscpu output
func CPUArchitecture(ctx context.Context, node *corev1.Node, logger *mylog.TestLogger) (string, error) {
	cpuInfo, err := lscpuPraser(ctx, node, logger)
	if err != nil {
		return "", fmt.Errorf("Unable to parse lscpu output")
	}
	for _, v := range cpuInfo.Lscpu {
		if v.Field == "Architecture:" {
			return v.Data, nil
		}
	}
	return "", fmt.Errorf("could not fetch CPU architecture")
}

// CPUVendorId returns Vendor ID information from lscpu output
func CPUVendorId(ctx context.Context, node *corev1.Node, logger *mylog.TestLogger) (string, error) {
	cpuInfo, err := lscpuPraser(ctx, node, logger)
	if err != nil {
		return "", fmt.Errorf("Unable to parse lscpu output")
	}
	for _, v := range cpuInfo.Lscpu {
		if v.Field == "Vendor ID:" {
			return v.Data, nil
		}
	}
	return "", fmt.Errorf("could not fetch CPU Vendor ID")
}

// IsCPUVendor checks if the CPU Vendor ID matches the given vendor string
func IsCPUVendor(ctx context.Context, node *corev1.Node, vendor string, logger *mylog.TestLogger) (bool, error) {
	vendorData, err := CPUVendorId(ctx, node, logger)
	if err != nil {
		return false, err
	}
	return vendorData == vendor, nil
}

// IsIntel returns if Vendor ID is GenuineIntel in lscpu output
func IsIntel(ctx context.Context, node *corev1.Node, logger *mylog.TestLogger) (bool, error) {
	isIntel, err := IsCPUVendor(ctx, node, IntelVendorID, logger)
	if err != nil {
		return false, err
	}
	return isIntel, nil
}

// IsAMD returns if Vendor ID is AuthenticAMD in lscpu output
func IsAMD(ctx context.Context, node *corev1.Node, logger *mylog.TestLogger) (bool, error) {
	isAMD, err := IsCPUVendor(ctx, node, AMDVendorID, logger)
	if err != nil {
		return false, err
	}
	return isAMD, nil
}

// IsARM returns if Architecture is aarch64
func IsARM(ctx context.Context, node *corev1.Node, logger *mylog.TestLogger) (bool, error) {
	architectureData, err := CPUArchitecture(ctx, node, logger)
	if err != nil {
		return false, err
	}

	return architectureData == "aarch64", nil
}
