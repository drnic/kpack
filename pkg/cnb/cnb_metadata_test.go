package cnb_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestMetadataRetriever(t *testing.T) {
	spec.Run(t, "Metadata Retriever", testMetadataRetriever)
}

func testMetadataRetriever(t *testing.T, when spec.G, it spec.S) {
	var (
		mockFactory = &registryfakes.FakeRemoteImageFactory{}
	)

	when("RemoteMetadataRetriever", func() {
		when("retrieving from a builder image", func() {
			it("gets buildpacks from a local image", func() {
				fakeImage := registryfakes.NewFakeRemoteImage("index.docker.io/builder/image", "sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
				fakeRunImage := registryfakes.NewFakeRemoteImage("foo.io/run", "sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
				err := fakeImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`)
				assert.NoError(t, err)

				imageRef := registry.NewNoAuthImageRef("test-repo-name")
				runImageRef := registry.NewNoAuthImageRef("foo.io/run:basecnb")

				mockFactory.NewRemoteReturnsOnCall(0, fakeImage, nil)
				mockFactory.NewRemoteReturnsOnCall(1, fakeRunImage, nil)

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: mockFactory}
				builderImage, err := subject.GetBuilderImage(imageRef)
				assert.NoError(t, err)

				require.Len(t, builderImage.BuilderBuildpackMetadata, 1)
				assert.Equal(t, builderImage.BuilderBuildpackMetadata[0].ID, "test.id")
				assert.Equal(t, builderImage.BuilderBuildpackMetadata[0].Version, "1.2.3")
				assert.Equal(t, builderImage.RunImage, "foo.io/run@sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
				assert.Equal(t, mockFactory.NewRemoteArgsForCall(0), imageRef)
				assert.Equal(t, mockFactory.NewRemoteArgsForCall(1), runImageRef)

				assert.Equal(t, "index.docker.io/builder/image@sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895", builderImage.Identifier)
			})
		})

		when("GetBuiltImage", func() {
			it("retrieves the metadata from the registry", func() {
				fakeImage := registryfakes.NewFakeRemoteImage("index.docker.io/built/image", "sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4")
				err := fakeImage.SetLabel("io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)
				assert.NoError(t, err)
				err = fakeImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"cloudfoundry/run:full-cnb"}}}`)
				assert.NoError(t, err)

				fakeImageRef := registry.NewNoAuthImageRef("built/image:tag")
				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: mockFactory}

				result, err := subject.GetBuiltImage(fakeImageRef)
				assert.NoError(t, err)

				metadata := result.BuildpackMetadata
				require.Len(t, metadata, 1)
				assert.Equal(t, metadata[0].ID, "test.id")
				assert.Equal(t, metadata[0].Version, "1.2.3")

				createdAtTime, err := fakeImage.CreatedAt()
				assert.NoError(t, err)

				assert.Equal(t, result.CompletedAt, createdAtTime)
				assert.Equal(t, result.Identifier, "index.docker.io/built/image@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4")
				assert.Equal(t, result.RunImage, "cloudfoundry/run@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0")
				assert.Equal(t, mockFactory.NewRemoteArgsForCall(0), fakeImageRef)
			})
		})
	})
}
