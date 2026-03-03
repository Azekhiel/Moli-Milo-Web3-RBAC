EL4042 (Encryption and Security) Final Project
Team:
1. William Gerald Briandelo
2. Syaugi Adhia Feriyaldi

# Moli-Milo: Full-Stack Web3 RBAC System

## Description
Moli-Milo is a robust Full-Stack Web3 application that integrates a Permissioned/Proof-of-Authority (PoA) Ethereum blockchain with a Go backend and a Next.js frontend. The core of this project is a Smart Contract-based Role-Based Access Control (RBAC) system featuring a strict 5-Layer Security Verification process. 

The security layers include:
1. AuthN (EIP-191 Wallet Signature)
2. Anti-Replay Attack Protection (Timestamp-based Nonce)
3. On-Chain Role Verification
4. Server-Side Time-Based Policy
5. On-Chain API Budget/Quota Check with Asynchronous Logging

## Tech Stack
* **Blockchain**: Geth (Go-Ethereum) with PoA (Clique) Consensus, Solidity, Hardhat, OpenZeppelin Contracts.
* **Backend**: Go (Golang), Gin Web Framework, go-ethereum (ethclient).
* **Frontend**: Next.js, React, Tailwind CSS, Ethers.js.

## Repository Structure
Below is the high-level structure of the repository:

/
|-- backend/                 # Go backend source code
|   |-- blockchain/          # Blockchain interaction logic
|   |-- config/              # Configuration and constants
|   |-- helpers/             # Utility functions and policies
|   |-- middleware/          # Authentication and 5-layer verification logic
|   |-- main.go              # Backend entry point
|   |-- client.go            # Test client for backend API
|-- chaindata/               # Geth local blockchain data and keystores
|-- smart-contracts/         # Solidity smart contracts and Hardhat environment
|   |-- contracts/           # AuditableRBAC.sol and others
|   |-- scripts/             # Deployment and setup scripts
|   |-- test/                # Smart contract test files
|-- attach.bat               # Script to attach Geth console
|-- buat-akun.bat            # Script to create a new Geth account
|-- init.bat                 # Script to initialize the genesis block
|-- run-node.bat             # Script to start the Geth node
|-- genesis-poa.json         # Genesis file for the PoA network

## Complete Tutorial: How to Run the Project

### Prerequisites
* Node.js (v16 or higher)
* Go (v1.19 or higher)
* Geth (Go-Ethereum) installed and added to your system PATH (1.10.x)
* MetaMask browser extension

#### Step 1: Initialize and Run the Local Blockchain (Geth)
We have provided batch scripts in the root directory to make the Geth node setup seamless.
1. Open your terminal at the root directory of the project.
2. Initialize the Genesis block for the PoA network:
   `init.bat`
3. Start the Geth node (automatically unlocks the miner/logger account and starts mining):
   `run-node.bat`
   *Note: Keep this terminal window open as it runs your local blockchain.*

#### Step 2: Smart Contract Deployment & Setup
All deployment and configuration tasks have been automated via batch scripts targeting the `gethDev` network.
1. Open a new terminal and navigate to the `smart-contracts` directory:
   `cd smart-contracts`
2. Install the necessary dependencies (first time only):
   `npm install`
3. Deploy the Smart Contract (`AuditableRBAC.sol`):
   `deploy.bat`
   *(Copy the deployed contract address from the output to update your backend/frontend if necessary).*
4. Grant access roles (Admin, Finance, Karyawan, Logger) to the respective user wallets:
   `grantRole.bat`
5. Setup the API budget/quotas for each role:
   `setupBudget.bat`

#### Step 3: Run the Backend Server
The Go backend handles the 5-Layer Security Verification and async blockchain logging.
1. Open a new terminal and navigate to the `backend` directory:
   `cd backend`
2. Install Go modules (first time only):
   `go mod tidy`
3. Start the Gin web server:
   `go run main.go`
   *The backend is now running on `http://localhost:8080`.*

#### Step 4: Run the Frontend Dashboard
The Next.js frontend is the interface for users to sign transactions via MetaMask.
1. Open a new terminal and navigate to the `frontend` directory:
   `cd frontend`
2. Install dependencies (first time only):
   `npm install`
3. Start the Next.js development server:
   `npm run dev`
4. Open your browser and access `http://localhost:3000`.

#### Step 5: Testing and Monitoring
1. **Connect Wallet:** Click "Connect Wallet" on the frontend. Make sure your MetaMask is connected to `Localhost 8545` (Chain ID: 1337).
2. **Interact:** Click any of the role cards (e.g., Area Karyawan). MetaMask will prompt you to sign a message (`Moli-Milo_Auth_Nonce_{timestamp}`).
3. **Check Blockchain Logs:** To verify that the backend successfully wrote the audit trail to the blockchain, open a terminal at the root directory and run:
   `log.bat`
   *This will fetch and display all `AccessLogged` events directly from the Geth IPC.*