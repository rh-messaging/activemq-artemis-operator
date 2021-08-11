package channels

import (
	"k8s.io/apimachinery/pkg/types"
)

//This channel is used to receive new ready pods
//moved here to void cyclic imports
var AddressListeningCh = make(chan types.NamespacedName)
