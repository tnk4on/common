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

	res, size, err = runtime.DiskUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, initialTotalImageSize+int64(len(manifest)), size)
	require.Len(t, res, 2)

	var baseImageUsage, syntheticImageUsage *ImageDiskUsage
	for i := range res {
		res[i].Created = time.Time{}
		require.Equal(t, res[i].SharedSize+res[i].UniqueSize, res[i].Size)
		switch res[i].ID {
		case imgID:
			baseImageUsage = &res[i]
		case img2.ID:
			syntheticImageUsage = &res[i]
		}
	}

	require.NotNil(t, baseImageUsage)
	require.NotNil(t, syntheticImageUsage)
	require.Equal(t, "quay.io/libpod/alpine", baseImageUsage.Repository)
	require.Equal(t, "3.10.2", baseImageUsage.Tag)
	require.Equal(t, "localhost/test", syntheticImageUsage.Repository)
	require.Equal(t, "123", syntheticImageUsage.Tag)
	require.Equal(t, int64(len(manifest)), syntheticImageUsage.UniqueSize)
	require.Equal(t, baseImageUsage.SharedSize, syntheticImageUsage.SharedSize)
}
