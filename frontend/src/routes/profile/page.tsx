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
      <div className="border-t border-border pt-8">
        <EditorPreferenceCard />
      </div>
      <div className="border-t border-border pt-8">
        <PasswordForm />
      </div>
    </div>
  );
}
