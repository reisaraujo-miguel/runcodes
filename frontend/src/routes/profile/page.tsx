import { PageHeader } from "@/components/app/PageHeader";
import { EditorPreferenceCard } from "@/components/profile/EditorPreferenceCard";
import { PasswordForm } from "@/components/profile/PasswordForm";
import { ProfileForm } from "@/components/profile/ProfileForm";

export function ProfilePage() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Meu Perfil"
        description="Gerencie os seus dados de conta, preferências e senha."
      />
      <ProfileForm />
      <EditorPreferenceCard />
      <PasswordForm />
    </div>
  );
}
