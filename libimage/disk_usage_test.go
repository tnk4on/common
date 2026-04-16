//go:build !remote

package libimage

import (
	"context"
	"testing"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/storage"
	"github.com/stretchr/testify/require"
)

func TestDiskUsage(t *testing.T) {
	runtime := testNewRuntime(t)
	ctx := context.Background()

	name := "quay.io/libpod/alpine:3.10.2"
	pullOptions := &PullOptions{}
	pulledImages, err := runtime.Pull(ctx, name, config.PullPolicyAlways, pullOptions)
	require.NoError(t, err)
	require.Len(t, pulledImages, 1)
	imgID := pulledImages[0].storageImage.ID
	layerID := pulledImages[0].storageImage.TopLayer
	digest := pulledImages[0].storageImage.Digest
	img, err := pulledImages[0].storageReference.NewImageSource(ctx, &runtime.systemContext)
	require.NoError(t, err)
	defer img.Close()
	manifest, _, err := img.GetManifest(ctx, nil)
	require.NoError(t, err)

	res, size, err := runtime.DiskUsage(ctx)
	require.NoError(t, err)
	require.Len(t, res, 1)
	initialTotalImageSize := size

	expectedImageDiskUsage := ImageDiskUsage{
		ID:         imgID,
		Repository: "quay.io/libpod/alpine",
		Tag:        "3.10.2",
		SharedSize: 0,
		UniqueSize: initialTotalImageSize,
		Size:       initialTotalImageSize,
	}

	// intentionally unsetting the time here, we cannot really equal the time
	// because of the local information that is part of the struct and that
	// can differ even when the time is the same
	res[0].Created = time.Time{}
	require.Equal(t, expectedImageDiskUsage, res[0])

	opts := &storage.ImageOptions{
		BigData: []storage.ImageBigDataOption{
			{
				Key:    storage.ImageDigestBigDataKey,
				Data:   manifest,
				Digest: digest,
			},
		},
	}

	img2, err := runtime.store.CreateImage("", []string{"localhost/test:123"}, layerID, "", opts)
	require.NoError(t, err)

	sharedSize := initialTotalImageSize - int64(len(manifest))
	// copy the expected and update the expected values
	expectedImageDiskUsage2 := ImageDiskUsage{
		ID:         img2.ID,
		Repository: "localhost/test",
		Tag:        "123",
		SharedSize: sharedSize,
		UniqueSize: int64(len(manifest)),
		Size:       sharedSize + int64(len(manifest)),
	}
	expectedImageDiskUsage.SharedSize = sharedSize
	expectedImageDiskUsage.UniqueSize = expectedImageDiskUsage.Size - sharedSize

	res, size, err = runtime.DiskUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, initialTotalImageSize+int64(len(manifest)), size)
	require.Len(t, res, 2)
	res[0].Created = time.Time{}
	res[1].Created = time.Time{}
	require.ElementsMatch(t, []ImageDiskUsage{expectedImageDiskUsage, expectedImageDiskUsage2}, res)
}
