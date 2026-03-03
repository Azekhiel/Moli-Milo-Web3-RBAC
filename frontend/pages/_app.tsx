import '@/styles/globals.css';
import type { AppProps } from 'next/app';
import { Toaster } from 'react-hot-toast'; // <-- Impor notifikasi

export default function App({ Component, pageProps }: AppProps) {
  return (
    <>
      <Toaster
        position="bottom-right"
        toastOptions={{
          style: {
            background: '#333',
            color: '#fff',
          },
        }}
      />
      
      <Component {...pageProps} />
    </>
  );
}