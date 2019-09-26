package cnb

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/registry"
)

type ImageRebaser struct {
	RemoteImageFactory registry.RemoteImageFactory
}

func (f *ImageRebaser) Rebase(builderRef, previousImageRef registry.ImageRef) (BuiltImage, error) {
	builderImage, err := f.RemoteImageFactory.NewRemote(builderRef)
	if err != nil {
		return BuiltImage{}, err
	}

	previousImage, err := f.RemoteImageFactory.NewRemote(previousImageRef)
	if err != nil {
		return BuiltImage{}, err
	}

	var metadataJSON string
	metadataJSON, err = builderImage.Label(BuilderMetadataLabel)
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "builder image metadata label not present")
	}

	var metadata BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	runImage, err := f.RemoteImageFactory.NewRemote(runImageRef{
		namespace:  builderRef.Namespace(),
		image:      metadata.Stack.RunImage.Image,
		secretName: builderRef.SecretName(),
	})
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "unable to fetch remote run image")
	}

	rebasedImage, err := previousImage.Rebase("", runImage) //todo top layer needed
	if err != nil {
		return BuiltImage{}, err
	}

	return readBuiltImage(rebasedImage)
}

type runImageRef struct { //todo image ref helpers
	namespace  string
	image      string
	secretName string
}

func (r runImageRef) ServiceAccount() string {
	return ""
}

func (r runImageRef) Namespace() string {
	return r.namespace
}

func (r runImageRef) Image() string {
	return r.image
}

func (r runImageRef) HasSecret() bool {
	return r.secretName != ""
}

func (r runImageRef) SecretName() string {
	return r.secretName
}
