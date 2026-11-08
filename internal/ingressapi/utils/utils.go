package utils

import "fmt"

func NetIdToString(netID []byte) string { return fmt.Sprintf("%06X", netID) }
