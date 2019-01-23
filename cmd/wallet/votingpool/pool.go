package votingpool

import (
	"fmt"
	"sort"

	"git.parallelcoin.io/pod/pkg/txscript"
	"git.parallelcoin.io/pod/pkg/util"
	"git.parallelcoin.io/pod/pkg/util/hdkeychain"
<<<<<<< HEAD
	"git.parallelcoin.io/pod/module/wallet/waddrmgr"
	"git.parallelcoin.io/pod/module/wallet/walletdb"
	"git.parallelcoin.io/pod/module/wallet/zero"
=======
	"git.parallelcoin.io/pod/cmd/wallet/waddrmgr"
	"git.parallelcoin.io/pod/cmd/wallet/walletdb"
	"git.parallelcoin.io/pod/cmd/wallet/zero"
>>>>>>> master
)

const (
	minSeriesPubKeys = 3
	// CurrentVersion is the version used for newly created Series.
	CurrentVersion = 1
)

// Branch is the type used to represent a branch number in a series.
type Branch uint32

// Index is the type used to represent an index number in a series.
type Index uint32

// SeriesData represents a Series for a given Pool.
type SeriesData struct {
	version uint32
	// Whether or not a series is active. This is serialized/deserialized but
	// for now there's no way to deactivate a series.
	active bool
	// A.k.a. "m" in "m of n signatures needed".
	reqSigs     uint32
	publicKeys  []*hdkeychain.ExtendedKey
	privateKeys []*hdkeychain.ExtendedKey
}

// Pool represents an arrangement of notary servers to securely
// store and account for customer cryptocurrency deposits and to redeem
// valid withdrawals. For details about how the arrangement works, see
// http://opentransactions.org/wiki/index.php?title=Category:Voting_Pools
type Pool struct {
	ID           []byte
	seriesLookup map[uint32]*SeriesData
	manager      *waddrmgr.Manager
}

// PoolAddress represents a voting pool P2SH address, generated by
// deriving public HD keys from the series' master keys using the given
// branch/index and constructing a M-of-N multi-sig script.
type PoolAddress interface {
	SeriesID() uint32
	Branch() Branch
	Index() Index
}

type poolAddress struct {
	pool     *Pool
	addr     util.Address
	script   []byte
	seriesID uint32
	branch   Branch
	index    Index
}

// ChangeAddress is a votingpool address meant to be used on transaction change
// outputs. All change addresses have branch==0.
type ChangeAddress struct {
	*poolAddress
}

// WithdrawalAddress is a votingpool address that may contain unspent outputs
// available for use in a withdrawal.
type WithdrawalAddress struct {
	*poolAddress
}

// Create creates a new entry in the database with the given ID
// and returns the Pool representing it.
func Create(ns walletdb.ReadWriteBucket, m *waddrmgr.Manager, poolID []byte) (*Pool, error) {
	err := putPool(ns, poolID)
	if err != nil {
		str := fmt.Sprintf("unable to add voting pool %v to db", poolID)
		return nil, newError(ErrPoolAlreadyExists, str, err)
	}
	return newPool(m, poolID), nil
}

// Load fetches the entry in the database with the given ID and returns the Pool
// representing it.
func Load(ns walletdb.ReadBucket, m *waddrmgr.Manager, poolID []byte) (*Pool, error) {
	if !existsPool(ns, poolID) {
		str := fmt.Sprintf("unable to find voting pool %v in db", poolID)
		return nil, newError(ErrPoolNotExists, str, nil)
	}
	p := newPool(m, poolID)
	if err := p.LoadAllSeries(ns); err != nil {
		return nil, err
	}
	return p, nil
}

// newPool creates a new Pool instance.
func newPool(m *waddrmgr.Manager, poolID []byte) *Pool {
	return &Pool{
		ID:           poolID,
		seriesLookup: make(map[uint32]*SeriesData),
		manager:      m,
	}
}

// LoadAndGetDepositScript generates and returns a deposit script for the given seriesID,
// branch and index of the Pool identified by poolID.
func LoadAndGetDepositScript(ns walletdb.ReadBucket, m *waddrmgr.Manager, poolID string, seriesID uint32, branch Branch, index Index) ([]byte, error) {
	pid := []byte(poolID)
	p, err := Load(ns, m, pid)
	if err != nil {
		return nil, err
	}
	script, err := p.DepositScript(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	return script, nil
}

// LoadAndCreateSeries loads the Pool with the given ID, creating a new one if it doesn't
// yet exist, and then creates and returns a Series with the given seriesID, rawPubKeys
// and reqSigs. See CreateSeries for the constraints enforced on rawPubKeys and reqSigs.
func LoadAndCreateSeries(ns walletdb.ReadWriteBucket, m *waddrmgr.Manager, version uint32,
	poolID string, seriesID, reqSigs uint32, rawPubKeys []string) error {
	pid := []byte(poolID)
	p, err := Load(ns, m, pid)
	if err != nil {
		vpErr := err.(Error)
		if vpErr.ErrorCode == ErrPoolNotExists {
			p, err = Create(ns, m, pid)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return p.CreateSeries(ns, version, seriesID, reqSigs, rawPubKeys)
}

// LoadAndReplaceSeries loads the voting pool with the given ID and calls ReplaceSeries,
// passing the given series ID, public keys and reqSigs to it.
func LoadAndReplaceSeries(ns walletdb.ReadWriteBucket, m *waddrmgr.Manager, version uint32,
	poolID string, seriesID, reqSigs uint32, rawPubKeys []string) error {
	pid := []byte(poolID)
	p, err := Load(ns, m, pid)
	if err != nil {
		return err
	}
	return p.ReplaceSeries(ns, version, seriesID, reqSigs, rawPubKeys)
}

// LoadAndEmpowerSeries loads the voting pool with the given ID and calls EmpowerSeries,
// passing the given series ID and private key to it.
func LoadAndEmpowerSeries(ns walletdb.ReadWriteBucket, m *waddrmgr.Manager,
	poolID string, seriesID uint32, rawPrivKey string) error {
	pid := []byte(poolID)
	pool, err := Load(ns, m, pid)
	if err != nil {
		return err
	}
	return pool.EmpowerSeries(ns, seriesID, rawPrivKey)
}

// Series returns the series with the given ID, or nil if it doesn't
// exist.
func (p *Pool) Series(seriesID uint32) *SeriesData {
	series, exists := p.seriesLookup[seriesID]
	if !exists {
		return nil
	}
	return series
}

// Manager returns the waddrmgr.Manager used by this Pool.
func (p *Pool) Manager() *waddrmgr.Manager {
	return p.manager
}

// saveSeriesToDisk stores the given series ID and data in the database,
// first encrypting the public/private extended keys.
//
// This method must be called with the Pool's manager unlocked.
func (p *Pool) saveSeriesToDisk(ns walletdb.ReadWriteBucket, seriesID uint32, data *SeriesData) error {
	var err error
	encryptedPubKeys := make([][]byte, len(data.publicKeys))
	for i, pubKey := range data.publicKeys {
		encryptedPubKeys[i], err = p.manager.Encrypt(
			waddrmgr.CKTPublic, []byte(pubKey.String()))
		if err != nil {
			str := fmt.Sprintf("key %v failed encryption", pubKey)
			return newError(ErrCrypto, str, err)
		}
	}
	encryptedPrivKeys := make([][]byte, len(data.privateKeys))
	for i, privKey := range data.privateKeys {
		if privKey == nil {
			encryptedPrivKeys[i] = nil
		} else {
			encryptedPrivKeys[i], err = p.manager.Encrypt(
				waddrmgr.CKTPrivate, []byte(privKey.String()))
		}
		if err != nil {
			str := fmt.Sprintf("key %v failed encryption", privKey)
			return newError(ErrCrypto, str, err)
		}
	}

	err = putSeries(ns, p.ID, data.version, seriesID, data.active,
		data.reqSigs, encryptedPubKeys, encryptedPrivKeys)
	if err != nil {
		str := fmt.Sprintf("cannot put series #%d into db", seriesID)
		return newError(ErrSeriesSerialization, str, err)
	}
	return nil
}

// CanonicalKeyOrder will return a copy of the input canonically
// ordered which is defined to be lexicographical.
func CanonicalKeyOrder(keys []string) []string {
	orderedKeys := make([]string, len(keys))
	copy(orderedKeys, keys)
	sort.Sort(sort.StringSlice(orderedKeys))
	return orderedKeys
}

// Convert the given slice of strings into a slice of ExtendedKeys,
// checking that all of them are valid public (and not private) keys,
// and that there are no duplicates.
func convertAndValidatePubKeys(rawPubKeys []string) ([]*hdkeychain.ExtendedKey, error) {
	seenKeys := make(map[string]bool)
	keys := make([]*hdkeychain.ExtendedKey, len(rawPubKeys))
	for i, rawPubKey := range rawPubKeys {
		if _, seen := seenKeys[rawPubKey]; seen {
			str := fmt.Sprintf("duplicated public key: %v", rawPubKey)
			return nil, newError(ErrKeyDuplicate, str, nil)
		}
		seenKeys[rawPubKey] = true

		key, err := hdkeychain.NewKeyFromString(rawPubKey)
		if err != nil {
			str := fmt.Sprintf("invalid extended public key %v", rawPubKey)
			return nil, newError(ErrKeyChain, str, err)
		}

		if key.IsPrivate() {
			str := fmt.Sprintf("private keys not accepted: %v", rawPubKey)
			return nil, newError(ErrKeyIsPrivate, str, nil)
		}
		keys[i] = key
	}
	return keys, nil
}

// putSeries creates a new seriesData with the given arguments, ordering the
// given public keys (using CanonicalKeyOrder), validating and converting them
// to hdkeychain.ExtendedKeys, saves that to disk and adds it to this voting
// pool's seriesLookup map. It also ensures inRawPubKeys has at least
// minSeriesPubKeys items and reqSigs is not greater than the number of items in
// inRawPubKeys.
//
// This method must be called with the Pool's manager unlocked.
func (p *Pool) putSeries(ns walletdb.ReadWriteBucket, version, seriesID, reqSigs uint32, inRawPubKeys []string) error {
	if len(inRawPubKeys) < minSeriesPubKeys {
		str := fmt.Sprintf("need at least %d public keys to create a series", minSeriesPubKeys)
		return newError(ErrTooFewPublicKeys, str, nil)
	}

	if reqSigs > uint32(len(inRawPubKeys)) {
		str := fmt.Sprintf(
			"the number of required signatures cannot be more than the number of keys")
		return newError(ErrTooManyReqSignatures, str, nil)
	}

	rawPubKeys := CanonicalKeyOrder(inRawPubKeys)

	keys, err := convertAndValidatePubKeys(rawPubKeys)
	if err != nil {
		return err
	}

	data := &SeriesData{
		version:     version,
		active:      false,
		reqSigs:     reqSigs,
		publicKeys:  keys,
		privateKeys: make([]*hdkeychain.ExtendedKey, len(keys)),
	}

	err = p.saveSeriesToDisk(ns, seriesID, data)
	if err != nil {
		return err
	}
	p.seriesLookup[seriesID] = data
	return nil
}

// CreateSeries will create and return a new non-existing series.
//
// - seriesID must be greater than or equal 1;
// - rawPubKeys has to contain three or more public keys;
// - reqSigs has to be less or equal than the number of public keys in rawPubKeys.
func (p *Pool) CreateSeries(ns walletdb.ReadWriteBucket, version, seriesID, reqSigs uint32, rawPubKeys []string) error {
	if seriesID == 0 {
		return newError(ErrSeriesIDInvalid, "series ID cannot be 0", nil)
	}

	if series := p.Series(seriesID); series != nil {
		str := fmt.Sprintf("series #%d already exists", seriesID)
		return newError(ErrSeriesAlreadyExists, str, nil)
	}

	if seriesID != 1 {
		if _, ok := p.seriesLookup[seriesID-1]; !ok {
			str := fmt.Sprintf("series #%d cannot be created because series #%d does not exist",
				seriesID, seriesID-1)
			return newError(ErrSeriesIDNotSequential, str, nil)
		}
	}

	return p.putSeries(ns, version, seriesID, reqSigs, rawPubKeys)
}

// ActivateSeries marks the series with the given ID as active.
func (p *Pool) ActivateSeries(ns walletdb.ReadWriteBucket, seriesID uint32) error {
	series := p.Series(seriesID)
	if series == nil {
		str := fmt.Sprintf("series #%d does not exist, cannot activate it", seriesID)
		return newError(ErrSeriesNotExists, str, nil)
	}
	series.active = true
	err := p.saveSeriesToDisk(ns, seriesID, series)
	if err != nil {
		return err
	}
	p.seriesLookup[seriesID] = series
	return nil
}

// ReplaceSeries will replace an already existing series.
//
// - rawPubKeys has to contain three or more public keys
// - reqSigs has to be less or equal than the number of public keys in rawPubKeys.
func (p *Pool) ReplaceSeries(ns walletdb.ReadWriteBucket, version, seriesID, reqSigs uint32, rawPubKeys []string) error {
	series := p.Series(seriesID)
	if series == nil {
		str := fmt.Sprintf("series #%d does not exist, cannot replace it", seriesID)
		return newError(ErrSeriesNotExists, str, nil)
	}

	if series.IsEmpowered() {
		str := fmt.Sprintf("series #%d has private keys and cannot be replaced", seriesID)
		return newError(ErrSeriesAlreadyEmpowered, str, nil)
	}

	return p.putSeries(ns, version, seriesID, reqSigs, rawPubKeys)
}

// decryptExtendedKey uses Manager.Decrypt() to decrypt the encrypted byte slice and return
// an extended (public or private) key representing it.
//
// This method must be called with the Pool's manager unlocked.
func (p *Pool) decryptExtendedKey(keyType waddrmgr.CryptoKeyType, encrypted []byte) (*hdkeychain.ExtendedKey, error) {
	decrypted, err := p.manager.Decrypt(keyType, encrypted)
	if err != nil {
		str := fmt.Sprintf("cannot decrypt key %v", encrypted)
		return nil, newError(ErrCrypto, str, err)
	}
	result, err := hdkeychain.NewKeyFromString(string(decrypted))
	zero.Bytes(decrypted)
	if err != nil {
		str := fmt.Sprintf("cannot get key from string %v", decrypted)
		return nil, newError(ErrKeyChain, str, err)
	}
	return result, nil
}

// validateAndDecryptSeriesKeys checks that the length of the public and private key
// slices is the same, decrypts them, ensures the non-nil private keys have a matching
// public key and returns them.
//
// This function must be called with the Pool's manager unlocked.
func validateAndDecryptKeys(rawPubKeys, rawPrivKeys [][]byte, p *Pool) (pubKeys, privKeys []*hdkeychain.ExtendedKey, err error) {
	pubKeys = make([]*hdkeychain.ExtendedKey, len(rawPubKeys))
	privKeys = make([]*hdkeychain.ExtendedKey, len(rawPrivKeys))
	if len(pubKeys) != len(privKeys) {
		return nil, nil, newError(ErrKeysPrivatePublicMismatch,
			"the pub key and priv key arrays should have the same number of elements",
			nil)
	}

	for i, encryptedPub := range rawPubKeys {
		pubKey, err := p.decryptExtendedKey(waddrmgr.CKTPublic, encryptedPub)
		if err != nil {
			return nil, nil, err
		}
		pubKeys[i] = pubKey

		encryptedPriv := rawPrivKeys[i]
		var privKey *hdkeychain.ExtendedKey
		if encryptedPriv == nil {
			privKey = nil
		} else {
			privKey, err = p.decryptExtendedKey(waddrmgr.CKTPrivate, encryptedPriv)
			if err != nil {
				return nil, nil, err
			}
		}
		privKeys[i] = privKey

		if privKey != nil {
			checkPubKey, err := privKey.Neuter()
			if err != nil {
				str := fmt.Sprintf("cannot neuter key %v", privKey)
				return nil, nil, newError(ErrKeyNeuter, str, err)
			}
			if pubKey.String() != checkPubKey.String() {
				str := fmt.Sprintf("public key %v different than expected %v",
					pubKey, checkPubKey)
				return nil, nil, newError(ErrKeyMismatch, str, nil)
			}
		}
	}
	return pubKeys, privKeys, nil
}

// LoadAllSeries fetches all series (decrypting their public and private
// extended keys) for this Pool from the database and populates the
// seriesLookup map with them. If there are any private extended keys for
// a series, it will also ensure they have a matching extended public key
// in that series.
//
// This method must be called with the Pool's manager unlocked.
// FIXME: We should be able to get rid of this (and loadAllSeries/seriesLookup)
// by making Series() load the series data directly from the DB.
func (p *Pool) LoadAllSeries(ns walletdb.ReadBucket) error {
	series, err := loadAllSeries(ns, p.ID)
	if err != nil {
		return err
	}
	for id, series := range series {
		pubKeys, privKeys, err := validateAndDecryptKeys(
			series.pubKeysEncrypted, series.privKeysEncrypted, p)
		if err != nil {
			return err
		}
		p.seriesLookup[id] = &SeriesData{
			publicKeys:  pubKeys,
			privateKeys: privKeys,
			reqSigs:     series.reqSigs,
		}
	}
	return nil
}

// Change the order of the pubkeys based on branch number.
// Given the three pubkeys ABC, this would mean:
// - branch 0: CBA (reversed)
// - branch 1: ABC (first key priority)
// - branch 2: BAC (second key priority)
// - branch 3: CAB (third key priority)
func branchOrder(pks []*hdkeychain.ExtendedKey, branch Branch) ([]*hdkeychain.ExtendedKey, error) {
	if pks == nil {
		// This really shouldn't happen, but we want to be good citizens, so we
		// return an error instead of crashing.
		return nil, newError(ErrInvalidValue, "pks cannot be nil", nil)
	}

	if branch > Branch(len(pks)) {
		return nil, newError(
			ErrInvalidBranch, "branch number is bigger than number of public keys", nil)
	}

	if branch == 0 {
		numKeys := len(pks)
		res := make([]*hdkeychain.ExtendedKey, numKeys)
		copy(res, pks)
		// reverse pk
		for i, j := 0, numKeys-1; i < j; i, j = i+1, j-1 {
			res[i], res[j] = res[j], res[i]
		}
		return res, nil
	}

	tmp := make([]*hdkeychain.ExtendedKey, len(pks))
	tmp[0] = pks[branch-1]
	j := 1
	for i := 0; i < len(pks); i++ {
		if i != int(branch-1) {
			tmp[j] = pks[i]
			j++
		}
	}
	return tmp, nil
}

// DepositScriptAddress calls DepositScript to get a multi-signature
// redemption script and returns the pay-to-script-hash-address for that script.
func (p *Pool) DepositScriptAddress(seriesID uint32, branch Branch, index Index) (util.Address, error) {
	script, err := p.DepositScript(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	return p.addressFor(script)
}

func (p *Pool) addressFor(script []byte) (util.Address, error) {
	scriptHash := util.Hash160(script)
	return util.NewAddressScriptHashFromHash(scriptHash, p.manager.ChainParams())
}

// DepositScript constructs and returns a multi-signature redemption script where
// a certain number (Series.reqSigs) of the public keys belonging to the series
// with the given ID are required to sign the transaction for it to be successful.
func (p *Pool) DepositScript(seriesID uint32, branch Branch, index Index) ([]byte, error) {
	series := p.Series(seriesID)
	if series == nil {
		str := fmt.Sprintf("series #%d does not exist", seriesID)
		return nil, newError(ErrSeriesNotExists, str, nil)
	}

	pubKeys, err := branchOrder(series.publicKeys, branch)
	if err != nil {
		return nil, err
	}

	pks := make([]*util.AddressPubKey, len(pubKeys))
	for i, key := range pubKeys {
		child, err := key.Child(uint32(index))
		// TODO: implement getting the next index until we find a valid one,
		// in case there is a hdkeychain.ErrInvalidChild.
		if err != nil {
			str := fmt.Sprintf("child #%d for this pubkey %d does not exist", index, i)
			return nil, newError(ErrKeyChain, str, err)
		}
		pubkey, err := child.ECPubKey()
		if err != nil {
			str := fmt.Sprintf("child #%d for this pubkey %d does not exist", index, i)
			return nil, newError(ErrKeyChain, str, err)
		}
		pks[i], err = util.NewAddressPubKey(pubkey.SerializeCompressed(),
			p.manager.ChainParams())
		if err != nil {
			str := fmt.Sprintf(
				"child #%d for this pubkey %d could not be converted to an address",
				index, i)
			return nil, newError(ErrKeyChain, str, err)
		}
	}

	script, err := txscript.MultiSigScript(pks, int(series.reqSigs))
	if err != nil {
		str := fmt.Sprintf("error while making multisig script hash, %d", len(pks))
		return nil, newError(ErrScriptCreation, str, err)
	}

	return script, nil
}

// ChangeAddress returns a new votingpool address for the given seriesID and
// index, on the 0th branch (which is reserved for change addresses). The series
// with the given ID must be active.
func (p *Pool) ChangeAddress(seriesID uint32, index Index) (*ChangeAddress, error) {
	series := p.Series(seriesID)
	if series == nil {
		return nil, newError(ErrSeriesNotExists,
			fmt.Sprintf("series %d does not exist", seriesID), nil)
	}
	if !series.active {
		str := fmt.Sprintf("ChangeAddress must be on active series; series #%d is not", seriesID)
		return nil, newError(ErrSeriesNotActive, str, nil)
	}

	script, err := p.DepositScript(seriesID, Branch(0), index)
	if err != nil {
		return nil, err
	}
	pAddr, err := p.poolAddress(seriesID, Branch(0), index, script)
	if err != nil {
		return nil, err
	}
	return &ChangeAddress{poolAddress: pAddr}, nil
}

// WithdrawalAddress queries the address manager for the P2SH address
// of the redeem script generated with the given series/branch/index and uses
// that to populate the returned WithdrawalAddress. This is done because we
// should only withdraw from previously used addresses but also because when
// processing withdrawals we may iterate over a huge number of addresses and
// it'd be too expensive to re-generate the redeem script for all of them.
// This method must be called with the manager unlocked.
func (p *Pool) WithdrawalAddress(ns, addrmgrNs walletdb.ReadBucket, seriesID uint32, branch Branch, index Index) (
	*WithdrawalAddress, error) {
	// TODO: Ensure the given series is hot.
	addr, err := p.getUsedAddr(ns, addrmgrNs, seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	if addr == nil {
		str := fmt.Sprintf("cannot withdraw from unused addr (series: %d, branch: %d, index: %d)",
			seriesID, branch, index)
		return nil, newError(ErrWithdrawFromUnusedAddr, str, nil)
	}
	script, err := addr.Script()
	if err != nil {
		return nil, err
	}
	pAddr, err := p.poolAddress(seriesID, branch, index, script)
	if err != nil {
		return nil, err
	}
	return &WithdrawalAddress{poolAddress: pAddr}, nil
}

func (p *Pool) poolAddress(seriesID uint32, branch Branch, index Index, script []byte) (
	*poolAddress, error) {
	addr, err := p.addressFor(script)
	if err != nil {
		return nil, err
	}
	return &poolAddress{
			pool: p, seriesID: seriesID, branch: branch, index: index, addr: addr,
			script: script},
		nil
}

// EmpowerSeries adds the given extended private key (in raw format) to the
// series with the given ID, thus allowing it to sign deposit/withdrawal
// scripts. The series with the given ID must exist, the key must be a valid
// private extended key and must match one of the series' extended public keys.
//
// This method must be called with the Pool's manager unlocked.
func (p *Pool) EmpowerSeries(ns walletdb.ReadWriteBucket, seriesID uint32, rawPrivKey string) error {
	// make sure this series exists
	series := p.Series(seriesID)
	if series == nil {
		str := fmt.Sprintf("series %d does not exist for this voting pool",
			seriesID)
		return newError(ErrSeriesNotExists, str, nil)
	}

	// Check that the private key is valid.
	privKey, err := hdkeychain.NewKeyFromString(rawPrivKey)
	if err != nil {
		str := fmt.Sprintf("invalid extended private key %v", rawPrivKey)
		return newError(ErrKeyChain, str, err)
	}
	if !privKey.IsPrivate() {
		str := fmt.Sprintf(
			"to empower a series you need the extended private key, not an extended public key %v",
			privKey)
		return newError(ErrKeyIsPublic, str, err)
	}

	pubKey, err := privKey.Neuter()
	if err != nil {
		str := fmt.Sprintf("invalid extended private key %v, can't convert to public key",
			rawPrivKey)
		return newError(ErrKeyNeuter, str, err)
	}

	lookingFor := pubKey.String()
	found := false

	// Make sure the private key has the corresponding public key in the series,
	// to be able to empower it.
	for i, publicKey := range series.publicKeys {
		if publicKey.String() == lookingFor {
			found = true
			series.privateKeys[i] = privKey
		}
	}

	if !found {
		str := fmt.Sprintf(
			"private Key does not have a corresponding public key in this series")
		return newError(ErrKeysPrivatePublicMismatch, str, nil)
	}

	if err = p.saveSeriesToDisk(ns, seriesID, series); err != nil {
		return err
	}

	return nil
}

// EnsureUsedAddr ensures we have entries in our used addresses DB for the given
// seriesID, branch and all indices up to the given one. It must be called with
// the manager unlocked.
func (p *Pool) EnsureUsedAddr(ns, addrmgrNs walletdb.ReadWriteBucket, seriesID uint32, branch Branch, index Index) error {
	lastIdx, err := p.highestUsedIndexFor(ns, seriesID, branch)
	if err != nil {
		return err
	}
	if lastIdx == 0 {
		// highestUsedIndexFor() returns 0 when there are no used addresses for a
		// given seriesID/branch, so we do this to ensure there is an entry with
		// index==0.
		if err := p.addUsedAddr(ns, addrmgrNs, seriesID, branch, lastIdx); err != nil {
			return err
		}
	}
	lastIdx++
	for lastIdx <= index {
		if err := p.addUsedAddr(ns, addrmgrNs, seriesID, branch, lastIdx); err != nil {
			return err
		}
		lastIdx++
	}
	return nil
}

// addUsedAddr creates a deposit script for the given seriesID/branch/index,
// ensures it is imported into the address manager and finaly adds the script
// hash to our used addresses DB. It must be called with the manager unlocked.
func (p *Pool) addUsedAddr(ns, addrmgrNs walletdb.ReadWriteBucket, seriesID uint32, branch Branch, index Index) error {
	script, err := p.DepositScript(seriesID, branch, index)
	if err != nil {
		return err
	}

	// First ensure the address manager has our script. That way there's no way
	// to have it in the used addresses DB but not in the address manager.
	// TODO: Decide how far back we want the addr manager to rescan and set the
	// BlockStamp height according to that.
	manager, err := p.manager.FetchScopedKeyManager(waddrmgr.KeyScopeBIP0044)
	if err != nil {
		return err
	}
	_, err = manager.ImportScript(addrmgrNs, script, &waddrmgr.BlockStamp{})
	if err != nil && err.(waddrmgr.ManagerError).ErrorCode != waddrmgr.ErrDuplicateAddress {
		return err
	}

	encryptedHash, err := p.manager.Encrypt(waddrmgr.CKTPublic, util.Hash160(script))
	if err != nil {
		return newError(ErrCrypto, "failed to encrypt script hash", err)
	}
	err = putUsedAddrHash(ns, p.ID, seriesID, branch, index, encryptedHash)
	if err != nil {
		return newError(ErrDatabase, "failed to store used addr script hash", err)
	}

	return nil
}

// getUsedAddr gets the script hash for the given series, branch and index from
// the used addresses DB and uses that to look up the ManagedScriptAddress
// from the address manager. It must be called with the manager unlocked.
func (p *Pool) getUsedAddr(ns, addrmgrNs walletdb.ReadBucket, seriesID uint32, branch Branch, index Index) (
	waddrmgr.ManagedScriptAddress, error) {

	mgr := p.manager
	encryptedHash := getUsedAddrHash(ns, p.ID, seriesID, branch, index)
	if encryptedHash == nil {
		return nil, nil
	}
	hash, err := p.manager.Decrypt(waddrmgr.CKTPublic, encryptedHash)
	if err != nil {
		return nil, newError(ErrCrypto, "failed to decrypt stored script hash", err)
	}
	addr, err := util.NewAddressScriptHashFromHash(hash, mgr.ChainParams())
	if err != nil {
		return nil, newError(ErrInvalidScriptHash, "failed to parse script hash", err)
	}
	mAddr, err := mgr.Address(addrmgrNs, addr)
	if err != nil {
		return nil, err
	}
	return mAddr.(waddrmgr.ManagedScriptAddress), nil
}

// highestUsedIndexFor returns the highest index from this Pool's used addresses
// with the given seriesID and branch. It returns 0 if there are no used
// addresses with the given seriesID and branch.
func (p *Pool) highestUsedIndexFor(ns walletdb.ReadBucket, seriesID uint32, branch Branch) (Index, error) {
	return getMaxUsedIdx(ns, p.ID, seriesID, branch)
}

// String returns a string encoding of the underlying bitcoin payment address.
func (a *poolAddress) String() string {
	return a.addr.EncodeAddress()
}

func (a *poolAddress) addrIdentifier() string {
	return fmt.Sprintf("PoolAddress seriesID:%d, branch:%d, index:%d", a.seriesID, a.branch,
		a.index)
}

func (a *poolAddress) redeemScript() []byte {
	return a.script
}

func (a *poolAddress) series() *SeriesData {
	return a.pool.Series(a.seriesID)
}

func (a *poolAddress) SeriesID() uint32 {
	return a.seriesID
}

func (a *poolAddress) Branch() Branch {
	return a.branch
}

func (a *poolAddress) Index() Index {
	return a.index
}

// IsEmpowered returns true if this series is empowered (i.e. if it has
// at least one private key loaded).
func (s *SeriesData) IsEmpowered() bool {
	for _, key := range s.privateKeys {
		if key != nil {
			return true
		}
	}
	return false
}

func (s *SeriesData) getPrivKeyFor(pubKey *hdkeychain.ExtendedKey) (*hdkeychain.ExtendedKey, error) {
	for i, key := range s.publicKeys {
		if key.String() == pubKey.String() {
			return s.privateKeys[i], nil
		}
	}
	return nil, newError(ErrUnknownPubKey, fmt.Sprintf("unknown public key '%s'",
		pubKey.String()), nil)
}