package cosign

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"os"
	"sigstore-e2e-test/pkg/tas"
	"sigstore-e2e-test/pkg/tas/cosign"
	"sigstore-e2e-test/test/testSupport"
	"time"
)

const testImage string = "alpine:latest"

var cli *client.Client

var _ = Describe("Cosign test", Ordered, func() {
	var err error
	targetImageName := "ttl.sh/" + uuid.New().String() + ":5m"
	BeforeAll(func() {
		Expect(testSupport.InstallPrerequisites(
			tas.NewTas(testSupport.TestContext),
			cosign.NewCosign(testSupport.TestContext),
		)).To(Succeed())
		DeferCleanup(func() { testSupport.DestroyPrerequisites() })

		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		Expect(err).To(BeNil())

		var pull io.ReadCloser
		pull, err = cli.ImagePull(testSupport.TestContext, testImage, types.ImagePullOptions{})
		io.Copy(os.Stdout, pull)
		defer pull.Close()

		Expect(cli.ImageTag(testSupport.TestContext, testImage, targetImageName)).To(Succeed())
		var push io.ReadCloser
		push, err = cli.ImagePush(testSupport.TestContext, targetImageName, types.ImagePushOptions{RegistryAuth: types.RegistryAuthFromSpec})
		io.Copy(os.Stdout, push)
		defer push.Close()
		Expect(err).To(BeNil())
		// wait for a while to be sure that the image landed in the registry
		time.Sleep(10 * time.Second)
	})

	Describe("Cosign initialize", func() {
		It("should initialize the cosign root", func() {
			Expect(cosign.Cosign(testSupport.TestContext,
				"initialize",
				"--mirror="+tas.TufURL,
				"--root="+tas.TufURL+"/root.json")).To(Succeed())
		})
	})

	Describe("cosign sign", func() {
		It("should sign the container", func() {
			token, err := testSupport.GetOIDCToken(tas.OidcIssuerURL, "jdoe", "secure", tas.OIDC_REALM)
			Expect(err).To(BeNil())
			Expect(err).To(BeNil())
			Expect(cosign.Cosign(testSupport.TestContext,
				"sign", "-y", "--fulcio-url="+tas.FulcioURL, "--rekor-url="+tas.RekorURL, "--oidc-issuer="+tas.OidcIssuerURL, "--identity-token="+token, targetImageName)).To(Succeed())
		})
	})

	Describe("cosign verify", func() {
		It("should verify the signature", func() {
			Expect(cosign.Cosign(testSupport.TestContext, "verify", "--rekor-url="+tas.RekorURL, "--certificate-identity-regexp", ".*@redhat", "--certificate-oidc-issuer-regexp", ".*keycloak.*", targetImageName)).To(Succeed())
		})
	})
})