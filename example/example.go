package example

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"

	"github.com/taurusgroup/cmp-ecdsa/pkg/message"
	"github.com/taurusgroup/cmp-ecdsa/pkg/party"
	"github.com/taurusgroup/cmp-ecdsa/pkg/protocol"
	"github.com/taurusgroup/cmp-ecdsa/protocols/cmp/keygen"
	"github.com/taurusgroup/cmp-ecdsa/protocols/cmp/sign"
	"github.com/taurusgroup/cmp-ecdsa/protocols/example/xor"
)

func XOR(id party.ID, ids party.IDSlice, n Network) error {
	h, err := protocol.NewHandler(xor.StartXOR(id, ids))
	if err != nil {
		return err
	}
	err = handlerLoop(id, h, n)
	if err != nil {
		return err
	}
	r, err := h.Result()
	if err != nil {
		return err
	}
	h.Log.Info().Hex("xor", r.(xor.Result)).Msg("XOR result")
	return nil
}

func Keygen(id party.ID, ids party.IDSlice, threshold int, n Network) (*keygen.Result, error) {
	// KEYGEN
	h, err := protocol.NewHandler(keygen.StartKeygen(ids, threshold, id))
	if err != nil {
		return nil, err
	}
	err = handlerLoop(id, h, n)
	if err != nil {
		return nil, err
	}
	r, err := h.Result()
	if err != nil {
		return nil, err
	}

	return r.(*keygen.Result), nil
}

func Refresh(keygenResult *keygen.Result, n Network) (*keygen.Result, error) {
	hRefresh, err := protocol.NewHandler(keygen.StartRefresh(keygenResult.Session, keygenResult.Secret))
	if err != nil {
		return nil, err
	}
	err = handlerLoop(keygenResult.Secret.ID, hRefresh, n)
	if err != nil {
		return nil, err
	}

	r, err := hRefresh.Result()
	if err != nil {
		return nil, err
	}

	return r.(*keygen.Result), nil
}

func Sign(refreshResult *keygen.Result, m []byte, signers party.IDSlice, n Network) error {
	h, err := protocol.NewHandler(sign.StartSign(refreshResult.Session, refreshResult.Secret, signers, m))
	if err != nil {
		return err
	}
	err = handlerLoop(refreshResult.Secret.ID, h, n)
	if err != nil {
		return err
	}

	signResult, err := h.Result()
	if err != nil {
		return err
	}
	signature := signResult.(*sign.Result).Signature
	r, s := signature.ToRS()
	if !ecdsa.Verify(refreshResult.Session.PublicKey(), m, r, s) {
		return errors.New("signature failed to verify")
	}
	return nil
}

func All(id party.ID, ids party.IDSlice, threshold int, message []byte, n Network, wg *sync.WaitGroup) error {
	defer wg.Done()

	// XOR
	err := XOR(id, ids, n)
	if err != nil {
		return err
	}

	// KEYGEN
	keygenResult, err := Keygen(id, ids, threshold, n)
	if err != nil {
		return err
	}

	// REFRESH
	refreshResult, err := Refresh(keygenResult, n)
	if err != nil {
		return err
	}

	// SIGN
	signers := ids[:threshold+1]
	if !signers.Contains(id) {
		return nil
	}
	err = Sign(refreshResult, message, signers, n)
	if err != nil {
		return err
	}

	return nil
}

func handlerLoop(id party.ID, h *protocol.Handler, network Network) error {
	for {
		select {

		// outgoing messages
		case msg, ok := <-h.Listen():
			if !ok {
				// the channel was closed, indicating that the protocol is done executing.
				return nil
			}
			network.Send(msg)

		// incoming messages
		case msg := <-network.Next(id):
			err := h.Update(msg)

			// a message.Error is not fatal and the message can be ignored
			if messageError := new(message.Error); errors.As(err, &messageError) {
				h.Log.Warn().Err(messageError).Msg("skipping message")
			}

			// a protocol.Error indicates that the protocol has aborted.
			// this error is also returned by h.Result()
			if protocolError := new(protocol.Error); errors.As(err, &protocolError) {
				h.Log.Error().Err(protocolError).Msg("protocol failed")
			}
		}
	}
}

func main() {
	ids := party.IDSlice{"a", "b", "c", "d", "e"}
	threshold := 4
	message_to_sign := []byte("hello")

	net := NewNetwork(ids)

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id party.ID) {
			if err := All(id, ids, threshold, message_to_sign, net, &wg); err != nil {
				fmt.Println(err)
			}
		}(id)
	}
	wg.Wait()
}