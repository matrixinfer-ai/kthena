package utils

import (
	"fmt"
	"hash"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/dump"
	"k8s.io/apimachinery/pkg/util/rand"
)

// HashRevision hashes the contents of revision's Data using FNV hashing.
// The returned hash will be a safe encoded string to avoid bad words.
func HashRevision(data runtime.RawExtension) string {
	hf := fnv.New32()
	if len(data.Raw) > 0 {
		_, err := hf.Write(data.Raw)
		if err != nil {
			return ""
		}
	}
	if data.Object != nil {
		DeepHashObject(hf, data.Object)
	}
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32()))
}

// DeepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	fmt.Fprintf(hasher, "%v", dump.ForHash(objectToWrite))
}
