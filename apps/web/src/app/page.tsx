import { AuthProvider } from "@/components/auth/auth-provider";
import { LifeHubClient } from "@/components/lifehub-client";

export default function HomePage() {
  return (
    <AuthProvider>
      <LifeHubClient />
    </AuthProvider>
  );
}
