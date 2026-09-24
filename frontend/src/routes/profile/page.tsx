import { Footer } from "@/components/Footer";
import { Navbar } from "@/components/Navbar";
import { PasswordForm } from "@/components/profile/PasswordForm";
import { ProfileForm } from "@/components/profile/ProfileForm";

/** Authenticated page where the user manages their own account. */
export function ProfilePage() {
  return (
    <div>
      <Navbar />
      <main>
        <div className="mx-auto max-w-3xl space-y-4 p-6">
          <h1 className="text-2xl font-semibold">Meu Perfil</h1>
          <ProfileForm />
          <PasswordForm />
        </div>
      </main>
      <Footer />
    </div>
  );
}
