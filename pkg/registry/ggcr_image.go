package registry

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
)

type GoContainerRegistryImage struct {
	image    v1.Image
	repoName string
	keychain authn.Keychain
}

func NewGoContainerRegistryImage(repoName string, keychain authn.Keychain) (*GoContainerRegistryImage, error) {
	image, err := newV1Image(keychain, repoName)
	if err != nil {
		return nil, err
	}

	ri := &GoContainerRegistryImage{
		repoName: repoName,
		image:    image,
		keychain: keychain,
	}

	return ri, nil
}

func newV1Image(keychain authn.Keychain, repoName string) (v1.Image, error) {
	var auth authn.Authenticator
	ref, err := name.ParseReference(repoName, name.WeakValidation)
	if err != nil {
		return nil, errors.Wrapf(err, "parse reference '%s'", repoName)
	}

	auth, err = keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving keychain for '%s'", ref.Context().Registry)
	}

	image, err := remote.Image(ref, remote.WithAuth(auth), remote.WithTransport(http.DefaultTransport))
	if err != nil {
		return nil, errors.Wrapf(err, "connect to registry store '%s'", repoName)
	}

	return image, nil
}

func (i *GoContainerRegistryImage) CreatedAt() (time.Time, error) {
	cfg, err := i.configFile()
	if err != nil {
		return time.Time{}, err
	}
	return cfg.Created.UTC(), nil
}

func (i *GoContainerRegistryImage) Env(key string) (string, error) {
	cfg, err := i.configFile()
	if err != nil {
		return "", err
	}
	for _, envVar := range cfg.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (i *GoContainerRegistryImage) Label(key string) (string, error) {
	cfg, err := i.configFile()
	if err != nil {
		return "", err
	}
	labels := cfg.Config.Labels
	return labels[key], nil
}

func (i *GoContainerRegistryImage) Identifier() (string, error) {
	ref, err := name.ParseReference(i.repoName, name.WeakValidation)
	if err != nil {
		return "", err
	}

	digest, err := i.image.Digest()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get digest for image '%s'", i.repoName)
	}

	return fmt.Sprintf("%s@%s", ref.Context().Name(), digest), nil
}

func (i *GoContainerRegistryImage) configFile() (*v1.ConfigFile, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get config for image '%s'", i.repoName)
	} else if cfg == nil {
		return nil, errors.Errorf("failed to get config for image '%s'", i.repoName)
	}

	return cfg, nil
}

func (i *GoContainerRegistryImage) Rebase(topLayer string, newBase RemoteImage) (RemoteImage, error) { //todo test this
	newBaseRemote, ok := newBase.(*GoContainerRegistryImage)
	if !ok {
		return nil, errors.New("expected new base to be a remote image")
	}

	newImage, err := mutate.Rebase(i.image, &subImage{img: i.image, topSHA: topLayer}, newBaseRemote.image)
	if err != nil {
		return nil, errors.Wrap(err, "rebase")
	}

	newImage, err = mutate.CreatedAt(newImage, v1.Time{Time: time.Now()})
	if err != nil {
		return nil, err
	}

	var auth authn.Authenticator
	ref, err := name.ParseReference(i.repoName, name.WeakValidation)
	if err != nil {
		return nil, errors.Wrapf(err, "parse reference '%s'", i.repoName)
	}

	auth, err = i.keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving keychain for '%s'", ref.Context().Registry)
	}

	if err := remote.Write(ref, newImage, remote.WithAuth(auth)); err != nil {
		return nil, err
	}

	return &GoContainerRegistryImage{
		image:    newImage,
		repoName: i.repoName,
	}, nil
}

type subImage struct {
	img    v1.Image
	topSHA string
}

func (si *subImage) Layers() ([]v1.Layer, error) {
	all, err := si.img.Layers()
	if err != nil {
		return nil, err
	}
	for i, l := range all {
		d, err := l.DiffID()
		if err != nil {
			return nil, err
		}
		if d.String() == si.topSHA {
			return all[:i+1], nil
		}
	}
	return nil, errors.New("could not find base layer in image")
}

func (si *subImage) Size() (int64, error)                    { panic("implement me") }
func (si *subImage) BlobSet() (map[v1.Hash]struct{}, error)  { panic("Not Implemented") }
func (si *subImage) MediaType() (types.MediaType, error)     { panic("Not Implemented") }
func (si *subImage) ConfigName() (v1.Hash, error)            { panic("Not Implemented") }
func (si *subImage) ConfigFile() (*v1.ConfigFile, error)     { panic("Not Implemented") }
func (si *subImage) RawConfigFile() ([]byte, error)          { panic("Not Implemented") }
func (si *subImage) Digest() (v1.Hash, error)                { panic("Not Implemented") }
func (si *subImage) Manifest() (*v1.Manifest, error)         { panic("Not Implemented") }
func (si *subImage) RawManifest() ([]byte, error)            { panic("Not Implemented") }
func (si *subImage) LayerByDigest(v1.Hash) (v1.Layer, error) { panic("Not Implemented") }
func (si *subImage) LayerByDiffID(v1.Hash) (v1.Layer, error) { panic("Not Implemented") }
