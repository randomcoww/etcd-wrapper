package s3util

import (
	"fmt"
	"strings"
)

func ParseBucketAndKey(path string) (string, string, error) {
	toks := strings.SplitN(path, "/", 2)
	if len(toks) != 2 || len(toks[0]) == 0 || len(toks[1]) == 0 {
		return "", "", fmt.Errorf("invalid S3 resource: %v", path)
	}
	return toks[0], toks[1], nil
}
