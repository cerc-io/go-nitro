import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  // https://vitejs.dev/guide/dep-pre-bundling.html#monorepos-and-linked-dependencies
  optimizeDeps: {
    include:  ["@statechannels/nitro-rpc-client"]
  },
  build: {
    commonjsOptions: {
      include: [/nitro-rpc-client/, /node_modules/],
    },
  }
});
