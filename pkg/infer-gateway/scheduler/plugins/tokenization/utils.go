package tokenization

import "encoding/binary"

func intToByteArray(tokens []int) []byte {
	result := make([]byte, len(tokens)*4)
	for i, token := range tokens {
		binary.BigEndian.PutUint32(result[i*4:(i+1)*4], uint32(token))
	}
	return result
}
