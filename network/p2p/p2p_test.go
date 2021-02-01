package p2p

import (
	"context"
	"sync"
	"testing"

	"github.com/bloxapp/ssv/ibft/proto"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/bloxapp/ssv/network"
)

func TestP2PNetworker(t *testing.T) {
	logger := zaptest.NewLogger(t)

	peer1, err := New(context.Background(), logger, "pub-key-1")
	require.NoError(t, err)

	peer2, err := New(context.Background(), logger, "pub-key-1")
	require.NoError(t, err)

	peer3, err := New(context.Background(), logger, "pub-key-1")
	require.NoError(t, err)

	peer4, err := New(context.Background(), logger, "pub-key-2")
	require.NoError(t, err)

	t.Run("peer 2 and peer 3 must receive messages from peer 1", func(t *testing.T) {
		messageToBroadcast := &proto.SignedMessage{
			Message: &proto.Message{
				Type:   proto.RoundState_PrePrepare,
				Round:  1,
				Lambda: []byte("test-lambda"),
				Value:  []byte("test-value"),
			},
		}

		var wg sync.WaitGroup

		var peer1Pipeline bool
		peer1.SetMessagePipeline(1, proto.RoundState_PrePrepare, []network.PipelineFunc{func(signedMessage *proto.SignedMessage) error {
			peer1Pipeline = true
			return nil
		}})

		wg.Add(1)
		var peer2Pipeline bool
		peer2.SetMessagePipeline(2, proto.RoundState_PrePrepare, []network.PipelineFunc{func(signedMessage *proto.SignedMessage) error {
			require.Equal(t, messageToBroadcast, signedMessage)
			peer2Pipeline = true
			wg.Done()
			return nil
		}})

		wg.Add(1)
		var peer3Pipeline bool
		peer3.SetMessagePipeline(3, proto.RoundState_PrePrepare, []network.PipelineFunc{func(signedMessage *proto.SignedMessage) error {
			require.Equal(t, messageToBroadcast, signedMessage)
			peer3Pipeline = true
			wg.Done()
			return nil
		}})

		var peer4Pipeline bool
		peer4.SetMessagePipeline(4, proto.RoundState_PrePrepare, []network.PipelineFunc{func(signedMessage *proto.SignedMessage) error {
			peer4Pipeline = true
			return nil
		}})

		err := peer1.Broadcast(messageToBroadcast)
		require.NoError(t, err)

		wg.Wait()

		require.False(t, peer1Pipeline)
		require.True(t, peer2Pipeline)
		require.True(t, peer3Pipeline)
		require.False(t, peer4Pipeline)
	})
}