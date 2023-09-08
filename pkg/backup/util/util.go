package util

import (
	"fmt"
	"strings"
)

// ParseBucketAndKey parses the path to return the s3 bucket name and key(path in the bucket)
// returns error if path is not in the format <s3-bucket-name>/<key>
func ParseBucketAndKey(path string) (string, string, error) {
	toks := strings.SplitN(path, "/", 2)
	if len(toks) != 2 || len(toks[0]) == 0 || len(toks[1]) == 0 {
		return "", "", fmt.Errorf("Invalid S3 path (%v)", path)
	}
	return toks[0], toks[1], nil
}
