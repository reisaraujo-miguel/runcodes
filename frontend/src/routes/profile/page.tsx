import { PageHeader } from "@/components/app/PageHeader";
import { PasswordForm } from "@/components/profile/PasswordForm";
import { ProfileForm } from "@/components/profile/ProfileForm";

export function ProfilePage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Meu Perfil"
        description="Gerencie os seus dados de conta e a sua senha."
      />
      <div className="max-w-2xl space-y-6">
        <ProfileForm />
        <PasswordForm />
      </div>
    </div>
  );
}
