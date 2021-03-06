// +build !js

package webrtc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

// If a remote doesn't support a Codec used by a `TrackLocalStatic`
// an error should be returned to the user
func Test_TrackLocalStatic_NoCodecIntersection(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
	assert.NoError(t, err)

	t.Run("Offerer", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		noCodecPC, err := NewAPI().NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		_, err = pc.AddTrack(track)
		assert.NoError(t, err)

		assert.True(t, errors.Is(signalPair(pc, noCodecPC), ErrUnsupportedCodec))

		assert.NoError(t, noCodecPC.Close())
		assert.NoError(t, pc.Close())
	})

	t.Run("Answerer", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		m := &MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP9", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo))

		vp9OnlyPC, err := NewAPI(WithMediaEngine(m)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		_, err = vp9OnlyPC.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		_, err = pc.AddTrack(track)
		assert.NoError(t, err)

		assert.True(t, errors.Is(signalPair(vp9OnlyPC, pc), ErrUnsupportedCodec))

		assert.NoError(t, vp9OnlyPC.Close())
		assert.NoError(t, pc.Close())
	})

	t.Run("Local", func(t *testing.T) {
		offerer, answerer, err := newPair()
		assert.NoError(t, err)

		invalidCodecTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/invalid-codec"}, "video", "pion")
		assert.NoError(t, err)

		_, err = offerer.AddTrack(invalidCodecTrack)
		assert.NoError(t, err)

		assert.True(t, errors.Is(signalPair(offerer, answerer), ErrUnsupportedCodec))
		assert.NoError(t, offerer.Close())
		assert.NoError(t, answerer.Close())
	})
}

// Assert that Bind/Unbind happens when expected
func Test_TrackLocalStatic_Closed(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	vp8Writer, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(vp8Writer)
	assert.NoError(t, err)

	assert.Equal(t, len(vp8Writer.bindings), 0, "No binding should exist before signaling")

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	assert.Equal(t, len(vp8Writer.bindings), 1, "binding should exist after signaling")

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())

	assert.Equal(t, len(vp8Writer.bindings), 0, "No binding should exist after close")
}

func Test_TrackLocalStatic_PayloadType(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	mediaEngineOne := &MediaEngine{}
	assert.NoError(t, mediaEngineOne.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        100,
	}, RTPCodecTypeVideo))

	mediaEngineTwo := &MediaEngine{}
	assert.NoError(t, mediaEngineTwo.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        200,
	}, RTPCodecTypeVideo))

	offerer, err := NewAPI(WithMediaEngine(mediaEngineOne)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerer, err := NewAPI(WithMediaEngine(mediaEngineTwo)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
	assert.NoError(t, err)

	_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = answerer.AddTrack(track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	offerer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		assert.Equal(t, track.PayloadType(), PayloadType(100))
		assert.Equal(t, track.Codec().RTPCodecCapability.MimeType, "video/VP8")

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{track})

	assert.NoError(t, offerer.Close())
	assert.NoError(t, answerer.Close())
}
