Kunci,Nilai Contoh,Tujuan dalam Proyek Cerberus

chainId,1337,ID Unik Jaringan Privat. Penting untuk mencegah transaksi replay dari jaringan lain.

difficulty,"""0x1""","Dibuat sangat rendah agar penambangan blok bisa dilakukan secara instan, yang ideal untuk pengembangan di localhost."

gasLimit,"""0x1FFFFFFFFFFF""",Ditetapkan sangat tinggi. Ini memberi banyak ruang gas untuk deployment dan interaksi Smart Contract RBAC yang kompleks tanpa khawatir gas habis.

alloc,{} (Diisi nanti),Alokasi Saldo Awal. Di sini akan ditambahkan alamat Ethereum yang dibuat agar memiliki Ether untuk membayar gas transaksi deployment kontrak.