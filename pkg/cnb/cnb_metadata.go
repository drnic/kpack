package cnb

import (
	"encoding/json"
	"strings"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/registry"
)

const BuilderMetadataLabel = "io.buildpacks.builder.metadata"

type BuildpackMetadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type BuilderImageMetadata struct {
	Buildpacks []BuildpackMetadata    `json:"buildpacks"`
	Stack      lcyclemd.StackMetadata `json:"stack"`
}

type Stack struct {
	RunImage RunImageRef `json:"runImage"`
}

type RunImageRef struct {
	Image string `json:"image"`
}

type BuilderImage struct {
	BuilderBuildpackMetadata BuilderMetadata
	RunImage                 string
	Identifier               string
}

type BuilderMetadata []BuildpackMetadata

type RemoteMetadataRetriever struct {
	RemoteImageFactory registry.RemoteImageFactory
}

func (r *RemoteMetadataRetriever) GetBuilderImage(repo registry.ImageRef) (BuilderImage, error) {
	img, err := r.RemoteImageFactory.NewRemote(repo)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	var metadataJSON string
	metadataJSON, err = img.Label(BuilderMetadataLabel)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "builder image metadata label not present")
	}

	var metadata BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	identifier, err := img.Identifier()
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "failed to retrieve builder image SHA")
	}

	runImage, err := r.RemoteImageFactory.NewRemote(registry.NewNoAuthImageRef(metadata.Stack.RunImage.Image))
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unable to fetch remote run image")
	}

	runImageIdentifier, err := runImage.Identifier()
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "failed to retrieve run image SHA")
	}

	return BuilderImage{
		BuilderBuildpackMetadata: metadata.Buildpacks,
		RunImage:                 runImageIdentifier,
		Identifier:               identifier,
	}, nil
}

func (r *RemoteMetadataRetriever) GetBuiltImage(ref registry.ImageRef) (BuiltImage, error) {
	img, err := r.RemoteImageFactory.NewRemote(ref)
	if err != nil {
		return BuiltImage{}, err
	}

	var buildMetadataJSON string
	var layerMetadataJSON string

	buildMetadataJSON, err = img.Label(lcyclemd.BuildMetadataLabel)
	if err != nil {
		return BuiltImage{}, err
	}

	layerMetadataJSON, err = img.Label(lcyclemd.LayerMetadataLabel)

	var buildMetadata lcyclemd.BuildMetadata
	var layerMetadata lcyclemd.LayersMetadata

	err = json.Unmarshal([]byte(buildMetadataJSON), &buildMetadata)
	if err != nil {
		return BuiltImage{}, err
	}

	err = json.Unmarshal([]byte(layerMetadataJSON), &layerMetadata)
	if err != nil {
		return BuiltImage{}, err
	}

	imageCreatedAt, err := img.CreatedAt()
	if err != nil {
		return BuiltImage{}, err
	}

	identifier, err := img.Identifier()
	if err != nil {
		return BuiltImage{}, err
	}

	runImageReference := layerMetadata.RunImage.Reference
	baseRunImage := layerMetadata.Stack.RunImage.Image
	runImage := strings.Split(baseRunImage, ":")[0] + "@" + strings.Split(runImageReference, "@")[1]

	return BuiltImage{
		Identifier:        identifier,
		CompletedAt:       imageCreatedAt,
		BuildpackMetadata: buildMetadata.Buildpacks,
		RunImage:          runImage,
	}, nil
}

type BuiltImage struct {
	Identifier        string
	CompletedAt       time.Time
	BuildpackMetadata []lcyclemd.BuildpackMetadata
	RunImage          string
}
