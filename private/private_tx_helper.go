package private

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"io"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
)

// SealedPrivatePayload represents a sealed payload entity in IPFS.
type SealedPrivatePayload struct {
	Payload       []byte
	SymmetricKeys [][]byte
	Participants  [][]byte
}

// NewSealedPrivatePayload creates new SealedPrivatePayload instance with given parameters.
func NewSealedPrivatePayload(encryptedPayload []byte, symmetricKey [][]byte, participants []*rsa.PublicKey) SealedPrivatePayload {
	keysToStore := make([][]byte, len(participants))
	for i, key := range participants {
		keysToStore[i] = x509.MarshalPKCS1PublicKey(key)
	}

	return SealedPrivatePayload{
		Payload:       encryptedPayload,
		SymmetricKeys: symmetricKey,
		Participants:  keysToStore,
	}
}

// toBytes returns serialized SealedPrivatePayload instance represented by bytes.
func (sealed *SealedPrivatePayload) toBytes() ([]byte, error) {
	bytes, err := rlp.EncodeToBytes(sealed)
	if err != nil {
		panic(err)
	}
	return bytes, nil
}

// PayloadReplacement represents the replacement data which substitute the private tx payload.
type PayloadReplacement struct {
	Participants []string
	Address      []byte
}

const defaultDbURL = "localhost:5001"

// SealPrivatePayload encrypts private tx's payload and send to IPFS, then replace the payload with the address in IPFS.
// Returns an address which could be used to retrieve original payload from IPFS.
func SealPrivatePayload(payload []byte, txNonce uint64, parties []string) (PayloadReplacement, error) {
	// Encrypt payload
	// use tx's nonce as gcm nonce
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce, txNonce)
	binary.BigEndian.PutUint32(nonce[8:], uint32(txNonce))
	encryptPayload, symKey, err := encryptPayload(payload, nonce)
	if err != nil {
		panic(err)
	}

	pubKeys, _ := stringsToPublicKeys(parties)

	// Encrypt symmetric keys for participants with related public key.
	symKeys := sealSymmetricKey(symKey, pubKeys)

	// Seal the payload by encrypting payload and appending symmetric key and participants.
	sealed := NewSealedPrivatePayload(encryptPayload, symKeys, pubKeys)

	// Put to IPFS
	ipfsDb := ethdb.NewIpfsDb(defaultDbURL)
	bytesToPut, _ := sealed.toBytes()
	ipfsAddr, err := ipfsDb.Put(bytesToPut)
	if err != nil {
		return PayloadReplacement{}, err
	}

	// Enclose as a PayloadReplacement struct.
	replacement := PayloadReplacement{
		Address:      ipfsAddr,
		Participants: parties,
	}
	return replacement, nil
}

// stringsToPublicKeys converts string to rsa.PublicKey instance.
func stringsToPublicKeys(keys []string) ([]*rsa.PublicKey, error) {
	pubKeys := make([]*rsa.PublicKey, len(keys))

	for i, p := range keys {
		if p[:2] != "0x" {
			p = "0x" + p
		}
		keyBuf, _ := hexutil.Decode(p)
		pubKeys[i], _ = x509.ParsePKCS1PublicKey(keyBuf)
	}
	return pubKeys, nil
}

// sealSymmetricKey sealed symmetric key by encrypting it with participant's public keys one by one.
func sealSymmetricKey(symKey []byte, keys []*rsa.PublicKey) [][]byte {
	result := make([][]byte, len(keys))
	for i, key := range keys {
		encryptedKey, err := rsa.EncryptPKCS1v15(rand.Reader, key, symKey)
		if err != nil {
			panic(err)
		}
		result[i] = encryptedKey
	}

	return result
}

const keyLength = 32

// generateSymmetricKey generate a random symmetric key.
func generateSymmetricKey() ([]byte, error) {
	key := make([]byte, keyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// encryptPayload encrypts payload with a random symmetric key and returns encrypted payload and the random symmetric key.
func encryptPayload(payload []byte, nonce []byte) (encryptedPayload []byte, symmetricKey []byte, err error) {
	symKey, err := generateSymmetricKey()
	if err != nil {
		panic(err)
	}

	block, err := aes.NewCipher(symKey)
	if err != nil {
		panic(err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}

	encrypted := aesgcm.Seal(nil, nonce, payload, nil)
	return encrypted, symKey, nil
}

// Below code is temporary and just for testing. It will be removed later.
// Test private keys and public keys in dev environment.
// Account1: e94b7b6c5a0e526a4d97f9768ad6097bde25c62a
//     private key: 0x308204a30201000282010100d065e5942da25a81fc431f46788281a19d2b961ca14cddc09376c7d63d949ae581735cbee1ff96d60b6410a4501d2c9df01ec6152e39600a80f0af1446c5f4ec275a292c5d9d1ef70a07c04c4f0dd1c8e586059002c16e9c4189c47c848adbd06f256a05da7557f3a4d781e7f185a47045eb4926c6db5c45f639091c7c3e1b29c9869f293b97963cdb83f586bf7e35d2ae1745c79baaa9912f2acd46b1fe35112c50eff32d356e6c2edc27dfa5564ad2ce04e8f39de86ddf5eb76e5958b23da580c242653463eec95ca186f916d5709ccae8ede25c1ad4b19cd62b1e1cfe7e6ea53f8fcd3c7812d2ceb89b5cd3e0d7d4926c9627ddd531fc59010b95a30de8a70203010001028201003b4901aac9e0aa06d890efd0c86fb81915f1545f08b42951a3a1e2efdbcceed3e3a3c1fabba84e6cce08c5833917539e0ab5767c880de2789a7dde10d2a1762fc87229cc69454d8dd1d8aaa80ac54faceb3ed94e42ba6c911f43e615d64efa81ad5ce3708ed95b1001111defb211e6d9d9ca39a142691d32f9fcf7ce96b9c457f7725ba60f8b83db5f8c9cafa419cb5e887518623733f41a7406afc2e193763f4ca714bec73df3514c82d4890b5b53650f5d2f72e0ad15180ee7809a2bc8ae18fdb7b9a525bdcb3a66ba9607c00c48791c71b6c51d058717af98d3e8ed72adcbcf0a023ab55ae5fe1845fb67195e9a558886854fc27f6b70a5382045f5ae074102818100df46f0c09eb444a5a2dfda893327682d4294543457a79f2d23cebd847def1b0ab8dac3662c459af1a1adc525254957c6580de7852bfe297b805b1e875cb0509c056ac05b9bab2c65b8204f8ffbeb190884113e3571014bceb73206efbd454779a51c0cab907ef24df5a0d176f238d247dc4aa41f7124ea11c2e6d8008de2ba3902818100eef0b69627e0e907eee0e7238b70ca206595b74f7ece36a05955f7ca50500628e74367bbb068918686f9185d05749b9ac916b683b2e3fb4554ba3d03691a1f1ac90c99c0aa881560600e5c3b7d64b48bce4d03ab8c51e28f6e48dd5c3778400a4cad76a76ac1a9b47da4c8586316a47143e3e944c9d9c6d82d844b39d3cf39df02818100a48062ceb7deff18be1889ad3e0811a40f02b3cb60ad7a044af67e0108bbcac3aa905b188313c165b7860cd322569819e534515877a229b3f94ca9007814db3f286a8f50af2f7d657034360a5243d34cc7e8e0598569bc0d90418684c9812a790061db1fe834ef96ea9ad2d8fcfb4a4a718e78bf45a039e85e1db0153074545902818023bca8f26860813a088666cbb02d5c6de003b6791354306367392e6879fe9e0d3c199ec839a84a2bbec03ede9ad447f9ac9dd30a7b95119ddb0047e3dcb26578921d6a59a0a7ddda9e434794363afbadf55b1b736af74c557b7f366c76776bcc9e8f4b31db0bc02018b2aeac5995a75eb172c30ee0c9cbadc59105d74e50ae2d0281801d051eae3e078597601839a55eedbca499c8e539a9da45a5c7de45b57c3fdeb0c8a2eb8bf34cf7511640fbfe9c4c3bdc824d6afea738890af633fe2a4d0223373010a3bc992094248e03355dfef0e04aa8b122e45e2b5fba27c4636bbe71d09d401625d62e70999d0cf0e509b8f09da683e5ab8350eff925f4e482aedce2c8f8
//     public key: 0x3082010a0282010100d065e5942da25a81fc431f46788281a19d2b961ca14cddc09376c7d63d949ae581735cbee1ff96d60b6410a4501d2c9df01ec6152e39600a80f0af1446c5f4ec275a292c5d9d1ef70a07c04c4f0dd1c8e586059002c16e9c4189c47c848adbd06f256a05da7557f3a4d781e7f185a47045eb4926c6db5c45f639091c7c3e1b29c9869f293b97963cdb83f586bf7e35d2ae1745c79baaa9912f2acd46b1fe35112c50eff32d356e6c2edc27dfa5564ad2ce04e8f39de86ddf5eb76e5958b23da580c242653463eec95ca186f916d5709ccae8ede25c1ad4b19cd62b1e1cfe7e6ea53f8fcd3c7812d2ceb89b5cd3e0d7d4926c9627ddd531fc59010b95a30de8a70203010001
//
// Account2: c05302acebd0730e3a18a058d7d1cb1204c4a092
//     private key: 0x308204a20201000282010100bc84262a13ceff4b5d3bfb296d594658ce52b2853d88df4393f96644cdb0c5ab8bf72d529422d955e046c225cf67cf311c3c32ca02abf9f0e3cf669dc702ae07fd234a953113c9744ef11bf33c9794e4b57742bcb2139edfdcc1fbc6258414ca4d9872ee59769aa8caecaa5495c891c168963fd6793e19a42e630f9265abaaf8374911c5ac5dc3170f122c5697fabc72fc4604523a4dd629a34510ade89a0eb26e9ad1ba56f0dfcc83294bcbda9b7d97b2e41d6ea2ad84957e4353207ac51753b801206b4ff99df96bcaec37728956b41ebe892eed87543cf41fba2b02401f15d6daa335baecd30f1622f8bf1bfd39ac638eee957dc3c30ed3b6d823708cd0470203010001028201003d7fcf03831ad06641b043abac24a7b268fcc988761ea4d762fac5c644641ad86ffcdf28457368fb7f03166b475252616f18a4690a9e1817e1f9d33c3da43e2a1506e259f17cc707ce8911d934372f37bd8b7e961872162e69d24ed4c1806957e62089be00299522e3b07990c69c7c1060924399304c7608fda90b7772fba1f66fbb558fd6a9731e11c1c2eae768c35d7b446ecfeab8e39f816eec48fa5004f8470994b6cd222de53a0d0effa848322999f622a5f5f54b37c9caf37c93ada421809a9d98f7356005124fe216c84c2873dc097ae597de41e66b112d00a087cf7e8dbaa76b52d70479f529b6ed4ce91831530119a5e979c499f40be7e8af43974102818100c4a18a07045ac2430b29ab121b82314a5335fb1bdf3de4549dbf247b2bbc99da0a9f99ee12e3e43828e18cb9ecba38b13fb172be861e15683eb9d490e8370342d4f599350c87ce49d8614391bdd8b9a3dc4fc38bb91d52cf3d5c06c9a1c1999cc7a87ccc64e27a94f956e40d11f9ec335c769c7ee3096381aa33eeaabfc6469902818100f56f61ad8e15c1c18469c41f0d5ba129ec0b8e8c67b26cd284b42b3ac0c7d0fdfcfbfcbd0f97fc7f22be08f6bbbc43c6eef65bfeb34ba600fb1c225e4019ca8257865a63156fe79aaa64191f0b338cd3aa8a77dde424ba580dbf3ee0aa0216dac04006a21cdf955714d9badb16575875af46260959114739567675b39c8f79df02818057fb1fd37bf35030c771e185bca14884c52ec628f67eaf07dd7d8549478ca01a9bde45f5eca5e39bed4edeb4e26380b26e996b8a2c60919b8f557ef347b435d5558c7efce99a6a8748365d117d2cd803a5b45afecdd97d10159873c10c8b9fbc32ea04cd3fe6c68a99f6731c160b09d10172611bb562a20f86a4ae09e0bd4b210281806ac3149e465c3878517d208ed164b66a61ff74f6a674fad9685867259b8e4fdeec19410b8ca8f470e94ff8de3b33ddd7bf42f3298c1cb00d652e0bd52bd50c3e3e8c76ecfafa3ea90ccd960fe6f379a2e9606a5bdf4e0ab11ae9c214405fc0494faf8a3322942f863dcfa8747cf769c767830030b8c9c74dadcac1d06b9e0dc90281807f205c52febc73105b273030a96c77b9a503d8b3aee0c1367ffe580a9ad98f42e492dfa2093d6e3417ebe58d92d3c939dd1e2636ef306406e99aa98d553b977c5724db0c28f2bb1ec0e54418fde7351bdbfe60bcadfa99e3cce3a870322a90724bf0ecf1284af605f6f66e92a7a8dbd674d6b60e04bbc0705fd75363f3404411
//     public key: 0x3082010a0282010100bc84262a13ceff4b5d3bfb296d594658ce52b2853d88df4393f96644cdb0c5ab8bf72d529422d955e046c225cf67cf311c3c32ca02abf9f0e3cf669dc702ae07fd234a953113c9744ef11bf33c9794e4b57742bcb2139edfdcc1fbc6258414ca4d9872ee59769aa8caecaa5495c891c168963fd6793e19a42e630f9265abaaf8374911c5ac5dc3170f122c5697fabc72fc4604523a4dd629a34510ade89a0eb26e9ad1ba56f0dfcc83294bcbda9b7d97b2e41d6ea2ad84957e4353207ac51753b801206b4ff99df96bcaec37728956b41ebe892eed87543cf41fba2b02401f15d6daa335baecd30f1622f8bf1bfd39ac638eee957dc3c30ed3b6d823708cd0470203010001
//
// Account3: ef3dd127de235f15ffb4fc0d71469d1339df6465
//     private key: 0x308204a30201000282010100c4e45de3f773a0dedf24cfb0b3e3944e64794d0c98134e0adc7c197ee23d85d768816280000eb048f09761ca38697f4c24e186d07ff4d797e221f697568496fe07cd329442b3783b1ffb1261dd9e33b7f5001275f1bbc25f77f1b693c640e6478fea87ec3a0675b8a45370c178d6e538a6e3ec53ede5fcfe7689c3b003b6cafb6545133ed90725a2a6f886d8bf9294bd683f563a16f9f30fedb528243e777acec8231160e07c08c55c894d55bc4d78d94b8abf33654494753ee210343e4f9f541b58de72713a34606092cb8f3d299c73e03ed3ae972bda36807180e0fde3720d8c2e196a526e2d643005ec08bcc9511202102b64a74ae9f2413aa5325426451702030100010282010100a566aed54658558944edb8a7d9c6d21cb4dd0df40981162b9ba3890b8565679d58c67087d50481e51470278f68aa7f6ce902a86d5940c72869a85c6e30193c7af4f4d58ba528fd54b5fe06283eb39b00eb897ef8a3f536495b0aac7521b3fd7f9a8fcc558f3d4401b3d200c4170e780b8a6fa865ad516aa21dd48796c2d7f955934a79c7c5d79986a0f3a1c97deb166b74fda6d8cfbbf0eb2c6cf25003f0d56ce8eee4ae6e16d1917d446250773490b49e22f99aa6e1ad08a79f1dd308d4ef85a399b7b717ca9b22fd1238f3872c7173bcd416e28d9f4724d68a850b69a39bbea51eaedf00bcb9756ca7216599a6de846f64830956f439cea725fa76eba6404902818100d125d1613bb628e38d5ebd534ad98029e7e6bee5c677a7230d642134c3b2bac55db4eaf828b8c0de4ad2b02c69cc767a87f52797b6e40c96ac9c2fca7df6d672924e84450504019bb142d2bf44c49b90fd08725fdfdf7562aeea1f2629010fa10b5026519d543810950e21dc3361054708fe6252ae6c03bc59c5637c87df72c302818100f0ffb649e26aae4f9ca4937f8d4cb424e472d13a4a7d0ec9b64ddc9a5ddff348d31fa36a53e7b813446836c6ac1dd89378a7363db58bb23486bba477478d4db9d0b9e988db1bc4c5b290fb64eca9b9dbb883636b9226b3f0b0ee70773478dcd39669bfadef06c33b89769073904d0f9346e2bebe5472a0a7413d6c2b3772571d0281803b241a8508418666723f6c01f594736d662a15a91bea11d513a050d37ed337853fee3cd3579086d9550726d22848ace81131fdb424ff6f9fdbc77eac1fda80e17d05bed95585c07eaa2d5f32bedb69b2221b155c8f0dbd3fde7e4db898b7b817adff4816a40a80a00fc623450532562fb4175aee4e6c34d23a005b1587c663c90281806257869439bf5ca801fcaa6fb742209499603cfeb35cbac7170c48c3f920a4cf07cff648323af143737baf367d0fa4cbf0c512fe3571eee33e439dc64abd5f853ea64ea4d8fc48dc7f9467f1741d824925ceffa7eab8be5eae646e224698374c64297cdd4617955d5b27b5a462b2ff7312cefe14feb2d3d9cc667b185b84de0502818068f219915d0b825941ab21f954c38188cf0bdc7be05fa750cf20d67178962deae2f8bbfe49032db65713cf66f72226a8f417a6c6374487e6ea973803dceeb7468aac975c3bf6ae642aeebee4b03bf984e9317ca3c8ad1ccbc6961084f70f0ccf0efb05765c3b1191b1ddc93a298aa36885ee53b7bcaf94f7ec7c3536716bd9c7
//     public key: 0x3082010a0282010100c4e45de3f773a0dedf24cfb0b3e3944e64794d0c98134e0adc7c197ee23d85d768816280000eb048f09761ca38697f4c24e186d07ff4d797e221f697568496fe07cd329442b3783b1ffb1261dd9e33b7f5001275f1bbc25f77f1b693c640e6478fea87ec3a0675b8a45370c178d6e538a6e3ec53ede5fcfe7689c3b003b6cafb6545133ed90725a2a6f886d8bf9294bd683f563a16f9f30fedb528243e777acec8231160e07c08c55c894d55bc4d78d94b8abf33654494753ee210343e4f9f541b58de72713a34606092cb8f3d299c73e03ed3ae972bda36807180e0fde3720d8c2e196a526e2d643005ec08bcc9511202102b64a74ae9f2413aa532542645170203010001
//
// Account4: 3a18598184ef84198db90c28fdfdfdf56544f747
//     private key: 0x308204a40201000282010100c41b896062c93243e178e11d146bdecdcacf06b7e561d57a245cebfbf8572864fb6b68556eb453bd66a5c9fef4247692ea3bd9dbb2ee8c1cc252ea3dff518fec37d9b240369bfc0d9708e776f0e3fc907a67f3c950839ffcb5f942114408efc9d931babcaef330106e3db1ec6fdc32ff3a8e2713b5ecc66efb786f857cb3f490093728f4262fbbaf800d55fda578331cfb4e4fde45a7770287498dc41af8efcacf9c7f892ef2933db57c76d7ab94d2d32c2edd18eb98bb5334110188565805d7b6438feb638d4a16d0fd8c24f869da373248e5b0cf8216d69715b5b164dcda884bdec9c7c74f4f1b8fc9f4b5973b8027590c67f410f5f41504bcb7e448edb7bb02030100010282010045e43e80c0944e3acd17e4bb157520721daecd092b5243e00527acdf1f7208ae7cc099eda0c7d9f46da9f6a4cbe456f22352f3610e9360123bdb8b2a4f5d853abde8f35359631c60c78c5fda0f1e61fc27f3f679b01d491eaac84c189533ce2a15235917380ee9f96120d1d19f484e509250e97267eb1c099fcc1b8aea97c43815a3c2aa816a3e4941748b3eeb34dfc2d132f4418357768d59cd6a46041a4dfaf9c6d0922fe38f9544f1c85bd3813fcab9a4c3786e72f37b8853fef14f8a5c956de378b64e4f2ffe401b117e807cd3993b547229a7c894841c3d8eb18d9dd13c2c1e3a7ec9898600f319fa3fcf1194f804224bcbde30b19b768da03f81bed07902818100dbf4899e1d4eb3887fc4ae1ee33eb1ec37f70f8be1895b4c8d2a551ae4b3424680b9f6b7cf4a4d3ad1c3e1cbaf2c16945599a15c7df66272425099ea6b3ae4a65d05d1cde225b424be3f70a756ae8223b324916fcfc98c6671bc4e2f2584eb700cd2782864f2b837e6c9457ab5a57c7d4d259cd6c7b4b2604e6b07abdd39ba7502818100e43e8d82fe34d93f699d56a0936d26660d722cfe627c1ae40faef709707d662c4f7aa34978c25a95ed632fe5fcf2028e5f5721871e4161ce58c5d0f12db9232afc876c173999251ab9532c08d98f5ece1e607952109074a432f73090432485511d8f5a85ae93870be4f206d68ed7a21f333c138c9f036080ae8c1919210c836f0281806aa73e7b9eb664b39150ab256b07217aeb002f57a27ad9fc5a8ee6496e0fc5d92dddfe55ce7bb6cb089fb4c2f123ada72b829d0d9e3e7429f721e2201af2a9a04986e2deb403984020c7de3625ffe436af4cd200a77e9147b36a9d769af8c2b8c85eddc8a87a50fd3a38ea29c01e8828b1d9c51d1824f4416284df696491f36902818100c0b978f0450a06ef1e94f652bc698be4dc31ae80565488b84dede536993fa9887ccc0718c0d90b78516c514397e419f871d4b6c0caf1564ed072a84d1dd8983371ec3f7f14e995850d3b87912973800ff7626acebaa1df7bce751f12913f433b0d04c0e0e45a39cbf753ce2659930697e5c13298a8a44756210cb71c9ae5600d02818100b8a6ecbd5de335b7205b9922a4a0f9eef1fccd76c8d7024839af95181c5d75cb42561f656e31b0784cb0d1489da7b8c9dc0d74416f8a586ab00a0db7cbe593da1e3eb39136e09820c9951a9d8180542694fd211ded0095d021d0fce058b1a68bb6dc7f787e162b51208ba373de910d790ab544f597d70b57a18e1be455b9f118
//     public key: 0x3082010a0282010100c41b896062c93243e178e11d146bdecdcacf06b7e561d57a245cebfbf8572864fb6b68556eb453bd66a5c9fef4247692ea3bd9dbb2ee8c1cc252ea3dff518fec37d9b240369bfc0d9708e776f0e3fc907a67f3c950839ffcb5f942114408efc9d931babcaef330106e3db1ec6fdc32ff3a8e2713b5ecc66efb786f857cb3f490093728f4262fbbaf800d55fda578331cfb4e4fde45a7770287498dc41af8efcacf9c7f892ef2933db57c76d7ab94d2d32c2edd18eb98bb5334110188565805d7b6438feb638d4a16d0fd8c24f869da373248e5b0cf8216d69715b5b164dcda884bdec9c7c74f4f1b8fc9f4b5973b8027590c67f410f5f41504bcb7e448edb7bb0203010001

var testKeyMap = make(map[string]*rsa.PrivateKey)

// GetPrivateKeyForAccount returns private key for given account. It is for testing purpose.
func GetPrivateKeyForAccount(account string) *rsa.PrivateKey {
	if len(testKeyMap) == 0 {
		// initialize
		d1, _ := hexutil.Decode("0x308204a30201000282010100d065e5942da25a81fc431f46788281a19d2b961ca14cddc09376c7d63d949ae581735cbee1ff96d60b6410a4501d2c9df01ec6152e39600a80f0af1446c5f4ec275a292c5d9d1ef70a07c04c4f0dd1c8e586059002c16e9c4189c47c848adbd06f256a05da7557f3a4d781e7f185a47045eb4926c6db5c45f639091c7c3e1b29c9869f293b97963cdb83f586bf7e35d2ae1745c79baaa9912f2acd46b1fe35112c50eff32d356e6c2edc27dfa5564ad2ce04e8f39de86ddf5eb76e5958b23da580c242653463eec95ca186f916d5709ccae8ede25c1ad4b19cd62b1e1cfe7e6ea53f8fcd3c7812d2ceb89b5cd3e0d7d4926c9627ddd531fc59010b95a30de8a70203010001028201003b4901aac9e0aa06d890efd0c86fb81915f1545f08b42951a3a1e2efdbcceed3e3a3c1fabba84e6cce08c5833917539e0ab5767c880de2789a7dde10d2a1762fc87229cc69454d8dd1d8aaa80ac54faceb3ed94e42ba6c911f43e615d64efa81ad5ce3708ed95b1001111defb211e6d9d9ca39a142691d32f9fcf7ce96b9c457f7725ba60f8b83db5f8c9cafa419cb5e887518623733f41a7406afc2e193763f4ca714bec73df3514c82d4890b5b53650f5d2f72e0ad15180ee7809a2bc8ae18fdb7b9a525bdcb3a66ba9607c00c48791c71b6c51d058717af98d3e8ed72adcbcf0a023ab55ae5fe1845fb67195e9a558886854fc27f6b70a5382045f5ae074102818100df46f0c09eb444a5a2dfda893327682d4294543457a79f2d23cebd847def1b0ab8dac3662c459af1a1adc525254957c6580de7852bfe297b805b1e875cb0509c056ac05b9bab2c65b8204f8ffbeb190884113e3571014bceb73206efbd454779a51c0cab907ef24df5a0d176f238d247dc4aa41f7124ea11c2e6d8008de2ba3902818100eef0b69627e0e907eee0e7238b70ca206595b74f7ece36a05955f7ca50500628e74367bbb068918686f9185d05749b9ac916b683b2e3fb4554ba3d03691a1f1ac90c99c0aa881560600e5c3b7d64b48bce4d03ab8c51e28f6e48dd5c3778400a4cad76a76ac1a9b47da4c8586316a47143e3e944c9d9c6d82d844b39d3cf39df02818100a48062ceb7deff18be1889ad3e0811a40f02b3cb60ad7a044af67e0108bbcac3aa905b188313c165b7860cd322569819e534515877a229b3f94ca9007814db3f286a8f50af2f7d657034360a5243d34cc7e8e0598569bc0d90418684c9812a790061db1fe834ef96ea9ad2d8fcfb4a4a718e78bf45a039e85e1db0153074545902818023bca8f26860813a088666cbb02d5c6de003b6791354306367392e6879fe9e0d3c199ec839a84a2bbec03ede9ad447f9ac9dd30a7b95119ddb0047e3dcb26578921d6a59a0a7ddda9e434794363afbadf55b1b736af74c557b7f366c76776bcc9e8f4b31db0bc02018b2aeac5995a75eb172c30ee0c9cbadc59105d74e50ae2d0281801d051eae3e078597601839a55eedbca499c8e539a9da45a5c7de45b57c3fdeb0c8a2eb8bf34cf7511640fbfe9c4c3bdc824d6afea738890af633fe2a4d0223373010a3bc992094248e03355dfef0e04aa8b122e45e2b5fba27c4636bbe71d09d401625d62e70999d0cf0e509b8f09da683e5ab8350eff925f4e482aedce2c8f8")
		prv1, _ := x509.ParsePKCS1PrivateKey(d1)

		d2, _ := hexutil.Decode("0x308204a20201000282010100bc84262a13ceff4b5d3bfb296d594658ce52b2853d88df4393f96644cdb0c5ab8bf72d529422d955e046c225cf67cf311c3c32ca02abf9f0e3cf669dc702ae07fd234a953113c9744ef11bf33c9794e4b57742bcb2139edfdcc1fbc6258414ca4d9872ee59769aa8caecaa5495c891c168963fd6793e19a42e630f9265abaaf8374911c5ac5dc3170f122c5697fabc72fc4604523a4dd629a34510ade89a0eb26e9ad1ba56f0dfcc83294bcbda9b7d97b2e41d6ea2ad84957e4353207ac51753b801206b4ff99df96bcaec37728956b41ebe892eed87543cf41fba2b02401f15d6daa335baecd30f1622f8bf1bfd39ac638eee957dc3c30ed3b6d823708cd0470203010001028201003d7fcf03831ad06641b043abac24a7b268fcc988761ea4d762fac5c644641ad86ffcdf28457368fb7f03166b475252616f18a4690a9e1817e1f9d33c3da43e2a1506e259f17cc707ce8911d934372f37bd8b7e961872162e69d24ed4c1806957e62089be00299522e3b07990c69c7c1060924399304c7608fda90b7772fba1f66fbb558fd6a9731e11c1c2eae768c35d7b446ecfeab8e39f816eec48fa5004f8470994b6cd222de53a0d0effa848322999f622a5f5f54b37c9caf37c93ada421809a9d98f7356005124fe216c84c2873dc097ae597de41e66b112d00a087cf7e8dbaa76b52d70479f529b6ed4ce91831530119a5e979c499f40be7e8af43974102818100c4a18a07045ac2430b29ab121b82314a5335fb1bdf3de4549dbf247b2bbc99da0a9f99ee12e3e43828e18cb9ecba38b13fb172be861e15683eb9d490e8370342d4f599350c87ce49d8614391bdd8b9a3dc4fc38bb91d52cf3d5c06c9a1c1999cc7a87ccc64e27a94f956e40d11f9ec335c769c7ee3096381aa33eeaabfc6469902818100f56f61ad8e15c1c18469c41f0d5ba129ec0b8e8c67b26cd284b42b3ac0c7d0fdfcfbfcbd0f97fc7f22be08f6bbbc43c6eef65bfeb34ba600fb1c225e4019ca8257865a63156fe79aaa64191f0b338cd3aa8a77dde424ba580dbf3ee0aa0216dac04006a21cdf955714d9badb16575875af46260959114739567675b39c8f79df02818057fb1fd37bf35030c771e185bca14884c52ec628f67eaf07dd7d8549478ca01a9bde45f5eca5e39bed4edeb4e26380b26e996b8a2c60919b8f557ef347b435d5558c7efce99a6a8748365d117d2cd803a5b45afecdd97d10159873c10c8b9fbc32ea04cd3fe6c68a99f6731c160b09d10172611bb562a20f86a4ae09e0bd4b210281806ac3149e465c3878517d208ed164b66a61ff74f6a674fad9685867259b8e4fdeec19410b8ca8f470e94ff8de3b33ddd7bf42f3298c1cb00d652e0bd52bd50c3e3e8c76ecfafa3ea90ccd960fe6f379a2e9606a5bdf4e0ab11ae9c214405fc0494faf8a3322942f863dcfa8747cf769c767830030b8c9c74dadcac1d06b9e0dc90281807f205c52febc73105b273030a96c77b9a503d8b3aee0c1367ffe580a9ad98f42e492dfa2093d6e3417ebe58d92d3c939dd1e2636ef306406e99aa98d553b977c5724db0c28f2bb1ec0e54418fde7351bdbfe60bcadfa99e3cce3a870322a90724bf0ecf1284af605f6f66e92a7a8dbd674d6b60e04bbc0705fd75363f3404411")
		prv2, _ := x509.ParsePKCS1PrivateKey(d2)

		d3, _ := hexutil.Decode("0x308204a30201000282010100c4e45de3f773a0dedf24cfb0b3e3944e64794d0c98134e0adc7c197ee23d85d768816280000eb048f09761ca38697f4c24e186d07ff4d797e221f697568496fe07cd329442b3783b1ffb1261dd9e33b7f5001275f1bbc25f77f1b693c640e6478fea87ec3a0675b8a45370c178d6e538a6e3ec53ede5fcfe7689c3b003b6cafb6545133ed90725a2a6f886d8bf9294bd683f563a16f9f30fedb528243e777acec8231160e07c08c55c894d55bc4d78d94b8abf33654494753ee210343e4f9f541b58de72713a34606092cb8f3d299c73e03ed3ae972bda36807180e0fde3720d8c2e196a526e2d643005ec08bcc9511202102b64a74ae9f2413aa5325426451702030100010282010100a566aed54658558944edb8a7d9c6d21cb4dd0df40981162b9ba3890b8565679d58c67087d50481e51470278f68aa7f6ce902a86d5940c72869a85c6e30193c7af4f4d58ba528fd54b5fe06283eb39b00eb897ef8a3f536495b0aac7521b3fd7f9a8fcc558f3d4401b3d200c4170e780b8a6fa865ad516aa21dd48796c2d7f955934a79c7c5d79986a0f3a1c97deb166b74fda6d8cfbbf0eb2c6cf25003f0d56ce8eee4ae6e16d1917d446250773490b49e22f99aa6e1ad08a79f1dd308d4ef85a399b7b717ca9b22fd1238f3872c7173bcd416e28d9f4724d68a850b69a39bbea51eaedf00bcb9756ca7216599a6de846f64830956f439cea725fa76eba6404902818100d125d1613bb628e38d5ebd534ad98029e7e6bee5c677a7230d642134c3b2bac55db4eaf828b8c0de4ad2b02c69cc767a87f52797b6e40c96ac9c2fca7df6d672924e84450504019bb142d2bf44c49b90fd08725fdfdf7562aeea1f2629010fa10b5026519d543810950e21dc3361054708fe6252ae6c03bc59c5637c87df72c302818100f0ffb649e26aae4f9ca4937f8d4cb424e472d13a4a7d0ec9b64ddc9a5ddff348d31fa36a53e7b813446836c6ac1dd89378a7363db58bb23486bba477478d4db9d0b9e988db1bc4c5b290fb64eca9b9dbb883636b9226b3f0b0ee70773478dcd39669bfadef06c33b89769073904d0f9346e2bebe5472a0a7413d6c2b3772571d0281803b241a8508418666723f6c01f594736d662a15a91bea11d513a050d37ed337853fee3cd3579086d9550726d22848ace81131fdb424ff6f9fdbc77eac1fda80e17d05bed95585c07eaa2d5f32bedb69b2221b155c8f0dbd3fde7e4db898b7b817adff4816a40a80a00fc623450532562fb4175aee4e6c34d23a005b1587c663c90281806257869439bf5ca801fcaa6fb742209499603cfeb35cbac7170c48c3f920a4cf07cff648323af143737baf367d0fa4cbf0c512fe3571eee33e439dc64abd5f853ea64ea4d8fc48dc7f9467f1741d824925ceffa7eab8be5eae646e224698374c64297cdd4617955d5b27b5a462b2ff7312cefe14feb2d3d9cc667b185b84de0502818068f219915d0b825941ab21f954c38188cf0bdc7be05fa750cf20d67178962deae2f8bbfe49032db65713cf66f72226a8f417a6c6374487e6ea973803dceeb7468aac975c3bf6ae642aeebee4b03bf984e9317ca3c8ad1ccbc6961084f70f0ccf0efb05765c3b1191b1ddc93a298aa36885ee53b7bcaf94f7ec7c3536716bd9c7")
		prv3, _ := x509.ParsePKCS1PrivateKey(d3)

		d4, _ := hexutil.Decode("0x308204a40201000282010100c41b896062c93243e178e11d146bdecdcacf06b7e561d57a245cebfbf8572864fb6b68556eb453bd66a5c9fef4247692ea3bd9dbb2ee8c1cc252ea3dff518fec37d9b240369bfc0d9708e776f0e3fc907a67f3c950839ffcb5f942114408efc9d931babcaef330106e3db1ec6fdc32ff3a8e2713b5ecc66efb786f857cb3f490093728f4262fbbaf800d55fda578331cfb4e4fde45a7770287498dc41af8efcacf9c7f892ef2933db57c76d7ab94d2d32c2edd18eb98bb5334110188565805d7b6438feb638d4a16d0fd8c24f869da373248e5b0cf8216d69715b5b164dcda884bdec9c7c74f4f1b8fc9f4b5973b8027590c67f410f5f41504bcb7e448edb7bb02030100010282010045e43e80c0944e3acd17e4bb157520721daecd092b5243e00527acdf1f7208ae7cc099eda0c7d9f46da9f6a4cbe456f22352f3610e9360123bdb8b2a4f5d853abde8f35359631c60c78c5fda0f1e61fc27f3f679b01d491eaac84c189533ce2a15235917380ee9f96120d1d19f484e509250e97267eb1c099fcc1b8aea97c43815a3c2aa816a3e4941748b3eeb34dfc2d132f4418357768d59cd6a46041a4dfaf9c6d0922fe38f9544f1c85bd3813fcab9a4c3786e72f37b8853fef14f8a5c956de378b64e4f2ffe401b117e807cd3993b547229a7c894841c3d8eb18d9dd13c2c1e3a7ec9898600f319fa3fcf1194f804224bcbde30b19b768da03f81bed07902818100dbf4899e1d4eb3887fc4ae1ee33eb1ec37f70f8be1895b4c8d2a551ae4b3424680b9f6b7cf4a4d3ad1c3e1cbaf2c16945599a15c7df66272425099ea6b3ae4a65d05d1cde225b424be3f70a756ae8223b324916fcfc98c6671bc4e2f2584eb700cd2782864f2b837e6c9457ab5a57c7d4d259cd6c7b4b2604e6b07abdd39ba7502818100e43e8d82fe34d93f699d56a0936d26660d722cfe627c1ae40faef709707d662c4f7aa34978c25a95ed632fe5fcf2028e5f5721871e4161ce58c5d0f12db9232afc876c173999251ab9532c08d98f5ece1e607952109074a432f73090432485511d8f5a85ae93870be4f206d68ed7a21f333c138c9f036080ae8c1919210c836f0281806aa73e7b9eb664b39150ab256b07217aeb002f57a27ad9fc5a8ee6496e0fc5d92dddfe55ce7bb6cb089fb4c2f123ada72b829d0d9e3e7429f721e2201af2a9a04986e2deb403984020c7de3625ffe436af4cd200a77e9147b36a9d769af8c2b8c85eddc8a87a50fd3a38ea29c01e8828b1d9c51d1824f4416284df696491f36902818100c0b978f0450a06ef1e94f652bc698be4dc31ae80565488b84dede536993fa9887ccc0718c0d90b78516c514397e419f871d4b6c0caf1564ed072a84d1dd8983371ec3f7f14e995850d3b87912973800ff7626acebaa1df7bce751f12913f433b0d04c0e0e45a39cbf753ce2659930697e5c13298a8a44756210cb71c9ae5600d02818100b8a6ecbd5de335b7205b9922a4a0f9eef1fccd76c8d7024839af95181c5d75cb42561f656e31b0784cb0d1489da7b8c9dc0d74416f8a586ab00a0db7cbe593da1e3eb39136e09820c9951a9d8180542694fd211ded0095d021d0fce058b1a68bb6dc7f787e162b51208ba373de910d790ab544f597d70b57a18e1be455b9f118")
		prv4, _ := x509.ParsePKCS1PrivateKey(d4)

		testKeyMap["e94b7b6c5a0e526a4d97f9768ad6097bde25c62a"] = prv1
		testKeyMap["c05302acebd0730e3a18a058d7d1cb1204c4a092"] = prv2
		testKeyMap["ef3dd127de235f15ffb4fc0d71469d1339df6465"] = prv3
		testKeyMap["3a18598184ef84198db90c28fdfdfdf56544f747"] = prv4
	}

	return testKeyMap[account]
}
