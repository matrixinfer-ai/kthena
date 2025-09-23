/*
Copyright The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tokenization

import "encoding/binary"

func intToByteArray(tokens []int) []byte {
	result := make([]byte, len(tokens)*4)
	for i, token := range tokens {
		binary.BigEndian.PutUint32(result[i*4:(i+1)*4], uint32(token))
	}
	return result
}
