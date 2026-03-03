package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Alamat Kontrak AuditableRBAC
var contractAddress = common.HexToAddress("0xa09369c24B596684d7dD4b16EaE418dd014FC413")
var contractABI abi.ABI
var ethClient *ethclient.Client

// Nonce Store
var usedNonces = make(map[string]map[int64]bool)

// Hash Role (Sesuai AuditableRBAC.sol)
var ADMIN_ROLE = crypto.Keccak256Hash([]byte("ADMIN_ROLE"))
var FINANCE_ROLE = crypto.Keccak256Hash([]byte("FINANCE_ROLE"))
var KARYAWAN_ROLE = crypto.Keccak256Hash([]byte("KARYAWAN_ROLE"))
var LOGGER_ROLE = crypto.Keccak256Hash([]byte("LOGGER_ROLE"))

// Kunci Privat & Signer untuk LOGGER
var loggerPrivateKey *ecdsa.PrivateKey
var loggerAddress common.Address = common.HexToAddress("0x153A5212b0eA63239021410dfa864c373a254F2c") // Akun Geth yang di-unlock
var chainID *big.Int

const minimalABI = `[
  { "constant": true, "inputs": [{ "internalType": "bytes32", "name": "role", "type": "bytes32" }, { "internalType": "address", "name": "account", "type": "address" }], "name": "hasRole", "outputs": [{ "internalType": "bool", "name": "", "type": "bool" }], "stateMutability": "view", "type": "function" },
  { "constant": true, "inputs": [{ "internalType": "bytes32", "name": "role", "type": "bytes32" }], "name": "checkBudget", "outputs": [{ "internalType": "bool", "name": "", "type": "bool" }], "stateMutability": "view", "type": "function" },
  { "constant": false, "inputs": [{ "internalType": "address", "name": "_user", "type": "address" }, { "internalType": "bytes32", "name": "_roleUsed", "type": "bytes32" }], "name": "logAccessAndDecrement", "outputs": [], "stateMutability": "nonpayable", "type": "function" },
  { "constant": false, "inputs": [{ "internalType": "bytes32", "name": "_role", "type": "bytes32" }, { "internalType": "uint256", "name": "_budget", "type": "uint256" }], "name": "setRoleBudget", "outputs": [], "stateMutability": "nonpayable", "type": "function" }
]`

// --- Struktur Request---
type AuthRequest struct {
	FromAddress string `json:"fromAddress" binding:"required"`
	Message     string `json:"message" binding:"required"`
	Signature   string `json:"signature" binding:"required"`
	Nonce       int64  `json:"nonce" binding:"required"`
}

func init() {
	var err error

	//  Koneksi Geth
	ethClient, err = ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatalf("Gagal terhubung ke Geth RPC: %v", err)
	}
	log.Println("Berhasil terhubung ke Geth RPC (http://localhost:8545).")

	// Memuat ABI
	contractABI, err = abi.JSON(strings.NewReader(minimalABI))
	if err != nil {
		log.Fatalf("Gagal memuat ABI: %v", err)
	}
	log.Printf("ABI Kontrak dimuat untuk %s.", contractAddress.Hex())

	// Memuat Private Key Logger
	const SENDER_PRIVATE_KEY = "0xf7dc752e87753bdfc58ef2ea378c2bc7f21a20da0a3cf386743f6d08c48cabd4"

	if SENDER_PRIVATE_KEY == "GANTI_DENGAN_PRIVATE_KEY_ANDA" {
		log.Fatal("ERROR: Harap ganti SENDER_PRIVATE_KEY di init() dengan kunci privat akun logger Anda.")
	}

	loggerPrivateKey, err = crypto.HexToECDSA(strings.TrimPrefix(SENDER_PRIVATE_KEY, "0x"))
	if err != nil {
		log.Fatalf("Gagal memuat kunci privat logger: %v", err)
	}
	loggerAddress = crypto.PubkeyToAddress(loggerPrivateKey.PublicKey)
	log.Printf("Logger Signer (Satpam) dimuat: %s", loggerAddress.Hex())

	// Mendapat Chain ID
	chainID, err = ethClient.ChainID(context.Background())
	if err != nil {
		log.Fatalf("Gagal mendapatkan Chain ID: %v", err)
	}
	log.Printf("Chain ID Geth: %v (Harusnya 1337).", chainID)
}

func main() {
	r := gin.Default()

	// Konfigurasi CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowHeaders = []string{"*"}
	r.Use(cors.New(config))

	// Endpoint Admin
	adminGroup := r.Group("/api/v1/admin")
	// Middleware ini hanya memastikan request memiliki payload AuthRequest
	adminGroup.Use(AuthRequestMiddleware())
	{
		adminGroup.POST("/dashboard", handleAdminDashboard) // Melihat Log & Status
		adminGroup.POST("/budget", handleSetRoleBudget)     // Mengatur Budget (Write Tx)
	}

	// Endpoint Finance
	financeGroup := r.Group("/api/v1/finance")
	financeGroup.Use(AuthRequestMiddleware())
	{
		financeGroup.POST("/laporan", handleFinanceData) // Melihat Laporan
	}

	// Endpoint Karyawan
	karyawanGroup := r.Group("/api/v1/karyawan")
	karyawanGroup.Use(AuthRequestMiddleware())
	{
		karyawanGroup.POST("/data", handleKaryawanData) // Akses Data Karyawan
	}

	log.Println("Menjalankan server API di http://localhost:8080 ...")
	r.Run(":8080")
}

// AuthRequestMiddleware membaca body, mengikatnya ke AuthRequest, dan menyimpannya di context.
func AuthRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Perlu untuk membaca raw body dua kali:
		// Untuk di-binding ke struct (Gin)
		// Untuk diverifikasi tandatangannya (EIP-191, butuh message mentah)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca body request."})
			return
		}

		// Reset body untuk handler berikutnya
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		var req AuthRequest
		if err := json.Unmarshal(body, &req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Format request tidak valid."})
			return
		}

		c.Set("authRequest", req)
		c.Set("rawMessage", req.Message)
		c.Next()
	}
}

func handleFinanceData(c *gin.Context) {
	req := c.MustGet("authRequest").(AuthRequest)

	recoveredAddress, err := run5Verifications(c, req, FINANCE_ROLE)
	if err != nil {
		return
	}

	// Lolos Verifikasi
	// Logging Asinkron
	go logAccessAsync(recoveredAddress, FINANCE_ROLE)

	log.Println("[Finance] Akses berhasil, kirim data laporan.")
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Akses Finance untuk %s berhasil!", recoveredAddress.Hex()),
		"role":    "FINANCE_ROLE",
		"laporan": "Laporan Keuangan Q3 (TERBATAS): $500,000",
	})
}

func handleKaryawanData(c *gin.Context) {
	req := c.MustGet("authRequest").(AuthRequest)

	recoveredAddress, err := run5Verifications(c, req, KARYAWAN_ROLE)
	if err != nil {
		return
	}

	// Lolos Verifikasi
	go logAccessAsync(recoveredAddress, KARYAWAN_ROLE)

	log.Println("[Karyawan] Akses berhasil, kirim data karyawan.")
	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Akses Karyawan untuk %s berhasil!", recoveredAddress.Hex()),
		"role":      "KARYAWAN_ROLE",
		"data_gaji": "Hanya Boleh Lihat Slip Gaji Pribadi",
	})
}

func handleAdminDashboard(c *gin.Context) {
	req := c.MustGet("authRequest").(AuthRequest)

	recoveredAddress, err := run5Verifications(c, req, ADMIN_ROLE)
	if err != nil {
		return
	}

	// Lolos Verifikasi
	go logAccessAsync(recoveredAddress, ADMIN_ROLE)

	log.Println("[Admin] Akses berhasil, kirim data Dashboard.")

	budgetFinance, _ := checkBudgetRemaining(FINANCE_ROLE)

	c.JSON(http.StatusOK, gin.H{
		"message":                  fmt.Sprintf("Akses Admin Penuh untuk %s!", recoveredAddress.Hex()),
		"role":                     "ADMIN_ROLE",
		"status":                   "Sistem Beroperasi",
		"budget_finance_remaining": budgetFinance,
	})
}

func handleSetRoleBudget(c *gin.Context) {
	req := c.MustGet("authRequest").(AuthRequest)

	type BudgetRequest struct {
		RoleName string `json:"roleName" binding:"required"`
		Budget   uint64 `json:"budget" binding:"required"`
	}
	var budgetReq BudgetRequest
	if err := c.ShouldBindJSON(&budgetReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload budget tidak valid."})
		return
	}

	// Menjalankan Verifikasi untuk role admin
	recoveredAddress, err := run5Verifications(c, req, ADMIN_ROLE)
	if err != nil {
		return
	}

	var targetRole common.Hash
	switch strings.ToUpper(budgetReq.RoleName) {
	case "FINANCE_ROLE":
		targetRole = FINANCE_ROLE
	case "KARYAWAN_ROLE":
		targetRole = KARYAWAN_ROLE
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama role tidak dikenal."})
		return
	}

	// Kirim Write Transaction: setRoleBudget (Signed by Logger)
	log.Printf("[Admin] Mengirim transaksi setRoleBudget untuk %s ke %d", budgetReq.RoleName, budgetReq.Budget)
	txHash, err := setRoleBudgetTx(targetRole, budgetReq.Budget)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengirim transaksi ke Geth: " + err.Error()})
		return
	}

	// Logging Asinkron untuk Admin
	go logAccessAsync(recoveredAddress, ADMIN_ROLE)

	c.JSON(http.StatusOK, gin.H{
		"message":          fmt.Sprintf("Budget %s berhasil diatur ke %d.", budgetReq.RoleName, budgetReq.Budget),
		"transaction_hash": txHash,
	})
}

// run5Verifications adalah helper yang menerapkan seluruh rantai keamanan.
func run5Verifications(c *gin.Context, req AuthRequest, requiredRole common.Hash) (common.Address, error) {
	//  Verifikasi #1 (AuthN) - Tanda Tangan EIP-191
	recoveredAddress, err := verifyEIP191Signature(req.Signature, req.Message, common.HexToAddress(req.FromAddress))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Verifikasi #1 GAGAL: Tanda Tangan tidak valid."})
		return common.Address{}, err
	}

	// Verifikasi #2 (Anti-Replay) - Nonce
	if !checkAndStoreNonce(recoveredAddress, req.Nonce) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Verifikasi #2 GAGAL: Nonce sudah pernah digunakan (Replay Attack)."})
		return common.Address{}, fmt.Errorf("nonce used")
	}

	// Verifikasi #3 (Role) - contract.hasRole (Read Call Geth)
	hasRole, err := checkHasRole(recoveredAddress, requiredRole)
	if err != nil || !hasRole {
		c.JSON(http.StatusForbidden, gin.H{"error": "Verifikasi #3 GAGAL: User tidak memiliki Role yang dibutuhkan."})
		return common.Address{}, fmt.Errorf("role access denied")
	}

	// Verifikasi #4 (Policy) - Jam Kerja (Server Side Logic)
	isPolicyValid := checkPolicyServerSide(requiredRole)
	if !isPolicyValid {
		c.JSON(http.StatusForbidden, gin.H{"error": "Verifikasi #4 GAGAL: Akses di luar jam kerja yang ditentukan (Policy)."})
		return common.Address{}, fmt.Errorf("policy access denied")
	}

	// 5. Verifikasi #5 (Budget) - contract.checkBudget (Read Call Geth)
	isBudgetAvailable, err := checkBudget(requiredRole)
	if err != nil || !isBudgetAvailable {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Verifikasi #5 GAGAL: Kuota akses habis (Budget)."})
		return common.Address{}, fmt.Errorf("budget exhausted")
	}

	return recoveredAddress, nil
}

// checkPolicyServerSide
func checkPolicyServerSide(role common.Hash) bool {
	// Atur zona waktu ke WIB (Asia/Jakarta)
	wibLocation, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Printf("[Policy] Gagal memuat lokasi WIB: %v", err)
		return false // Fail safe
	}
	nowInWIB := time.Now().In(wibLocation)
	currentHour := nowInWIB.Hour()

	roleHex := role.Hex()

	if roleHex == ADMIN_ROLE.Hex() {
		return true // Admin selalu boleh
	} else if roleHex == FINANCE_ROLE.Hex() {
		return currentHour >= 9 && currentHour < 20 // 08:00 - 19:59 WIB
	} else if roleHex == KARYAWAN_ROLE.Hex() {
		return currentHour >= 6 && currentHour < 17 // 08:00 - 16:59 WIB
	}
	return false
}

// checkHasRole (Read Call Geth)
func checkHasRole(user common.Address, role common.Hash) (bool, error) {
	callData, err := contractABI.Pack("hasRole", role, user)
	if err != nil {
		return false, err
	}

	msg := ethereum.CallMsg{To: &contractAddress, Data: callData}
	result, err := ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, err
	}

	var hasRole bool
	if err := contractABI.UnpackIntoInterface(&hasRole, "hasRole", result); err != nil {
		return false, err
	}
	return hasRole, nil
}

// checkBudget (Read Call Geth)
func checkBudget(role common.Hash) (bool, error) {
	callData, err := contractABI.Pack("checkBudget", role)
	if err != nil {
		return false, err
	}

	msg := ethereum.CallMsg{To: &contractAddress, Data: callData}
	result, err := ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return false, err
	}

	var isAvailable bool
	if err := contractABI.UnpackIntoInterface(&isAvailable, "checkBudget", result); err != nil {
		return false, err
	}
	return isAvailable, nil
}

// checkBudgetRemaining
func checkBudgetRemaining(role common.Hash) (uint64, error) {
	callData, err := contractABI.Pack("roleApiRemaining", role)
	if err != nil {
		return 0, err
	}

	msg := ethereum.CallMsg{To: &contractAddress, Data: callData}
	result, err := ethClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		return 0, err
	}

	// Unpack roleApiRemaining (uint256)
	var remaining *big.Int
	if err := contractABI.UnpackIntoInterface(&remaining, "roleApiRemaining", result); err != nil {
		return 0, err
	}
	return remaining.Uint64(), nil
}

// logAccessAsync mengirim transaksi log (Write Tx) di background.
func logAccessAsync(user common.Address, roleUsed common.Hash) {
	// Pastikan LOGGER_ROLE adalah penanda yang benar
	if loggerPrivateKey == nil || chainID == nil {
		log.Println("[Async Logger] GAGAL: Kunci privat atau ChainID belum dimuat.")
		return
	}

	// Dapatkan Nonce Geth
	nonce, err := ethClient.PendingNonceAt(context.Background(), loggerAddress)
	if err != nil {
		log.Printf("[Async Logger] GAGAL mendapatkan nonce: %v", err)
		return
	}

	// Packing Data (logAccessAndDecrement)
	callData, err := contractABI.Pack("logAccessAndDecrement", user, roleUsed)
	if err != nil {
		log.Printf("[Async Logger] GAGAL packing data: %v", err)
		return
	}

	// Perkiraan Gas
	gasLimit, err := ethClient.EstimateGas(context.Background(), ethereum.CallMsg{
		From: loggerAddress,
		To:   &contractAddress,
		Data: callData,
	})
	if err != nil {
		log.Printf("[Async Logger] GAGAL memperkirakan gas: %v", err)
		gasLimit = 300000 // Fallback
	}

	// Gas Price
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		log.Printf("[Async Logger] GAGAL mendapatkan Gas Price: %v", err)
		return
	}

	// Buat Transaksi
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &contractAddress,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     callData,
	})

	// Tandatangani Transaksi
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), loggerPrivateKey)
	if err != nil {
		log.Printf("[Async Logger] GAGAL menandatangani tx: %v", err)
		return
	}

	// Kirim Transaksi ke Geth
	err = ethClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Printf("[Async Logger] GAGAL mengirim tx: %v", err)
		return
	}

	log.Printf("[Async Logger] SUKSES! Transaksi logging terkirim. Hash: %s", signedTx.Hash().Hex())
}

// setRoleBudgetTx mengirim transaksi setRoleBudget (Write Tx)
func setRoleBudgetTx(role common.Hash, budget uint64) (string, error) {
	if loggerPrivateKey == nil || chainID == nil {
		return "", fmt.Errorf("Logger not initialized")
	}

	nonce, err := ethClient.PendingNonceAt(context.Background(), loggerAddress)
	if err != nil {
		return "", err
	}

	callData, err := contractABI.Pack("setRoleBudget", role, big.NewInt(int64(budget)))
	if err != nil {
		return "", err
	}

	gasLimit, err := ethClient.EstimateGas(context.Background(), ethereum.CallMsg{From: loggerAddress, To: &contractAddress, Data: callData})
	if err != nil {
		gasLimit = 300000
	}

	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		return "", err
	}

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &contractAddress,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     callData,
	})

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), loggerPrivateKey)
	if err != nil {
		return "", err
	}

	err = ethClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}

	return signedTx.Hash().Hex(), nil
}

// verifyEIP191Signature memverifikasi tanda tangan EIP-191 (standar ethers.js)
func verifyEIP191Signature(signatureHex string, message string, expectedAddress common.Address) (common.Address, error) {
	sig, err := hexutil.Decode(signatureHex)
	if err != nil {
		return common.Address{}, fmt.Errorf("gagal decode signature: %v", err)
	}

	// Ethers.js mengirim V dengan 27/28, Go-ethereum butuh 0/1
	if sig[64] == 27 || sig[64] == 28 {
		sig[64] -= 27
	}

	// Buat Hash yang sama seperti yang dibuat ethers.js
	eip191Message := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	msgHash := crypto.Keccak256Hash([]byte(eip191Message))

	pubkey, err := crypto.SigToPub(msgHash.Bytes(), sig)
	if err != nil {
		return common.Address{}, fmt.Errorf("gagal mendapatkan public key dari tanda tangan: %v", err)
	}

	recoveredAddress := crypto.PubkeyToAddress(*pubkey)

	if recoveredAddress.Hex() != expectedAddress.Hex() {
		return common.Address{}, fmt.Errorf("alamat yang dipulihkan (%s) tidak cocok dengan alamat yang diharapkan (%s)", recoveredAddress.Hex(), expectedAddress.Hex())
	}

	log.Printf("[EIP191] Verifikasi SUKSES untuk alamat: %s", recoveredAddress.Hex())
	return recoveredAddress, nil
}

// checkAndStoreNonce memeriksa nonce (Verifikasi #2)
func checkAndStoreNonce(addr common.Address, nonce int64) bool {
	addrHex := strings.ToLower(addr.Hex())
	if _, ok := usedNonces[addrHex]; !ok {
		usedNonces[addrHex] = make(map[int64]bool)
	}

	if usedNonces[addrHex][nonce] {
		log.Printf("[Nonce] GAGAL: Nonce %d sudah digunakan untuk %s", nonce, addrHex)
		return false
	}

	usedNonces[addrHex][nonce] = true
	log.Printf("[Nonce] SUKSES: Nonce %d disimpan untuk %s", nonce, addrHex)
	return true
}
