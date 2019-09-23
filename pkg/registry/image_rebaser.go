package registry

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type ImageRebaser struct {
	KeychainFactory KeychainFactory
}

func (f *ImageRebaser) Rebase(orig, oldBase, latestBase ImageRef) (RemoteImage, error) {
	origImage, err := NewGoContainerRegistryImage(orig.Image(), f.KeychainFactory.KeychainForImageRef(orig))
	if err != nil {
		return nil, err
	}

	oldBaseImage, err := NewGoContainerRegistryImage(oldBase.Image(), f.KeychainFactory.KeychainForImageRef(oldBase))
	if err != nil {
		return nil, err
	}

	latestBaseImage, err := NewGoContainerRegistryImage(latestBase.Image(), f.KeychainFactory.KeychainForImageRef(latestBase))
	if err != nil {
		return nil, err
	}

	rebase, err := mutate.Rebase(origImage.image, oldBaseImage.image, latestBaseImage.image)
	if err != nil {
		return nil, err
	}

	reference, err := name.ParseReference(origImage.repoName, name.WeakValidation)
	if err != nil {
		return nil, err
	}

	rebasedImage := &GoContainerRegistryImage{
		repoName: origImage.repoName,
		image:    rebase,
	}

	return rebasedImage, remote.Write(reference, rebase, remote.WithAuthFromKeychain(f.KeychainFactory.KeychainForImageRef(orig)))
}
