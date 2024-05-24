import { Component, Inject } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { UserService } from '../user.service';
import { User } from '../user';

@Component({
  selector: 'app-user-edit',
  templateUrl: './user-edit.component.html',
  styleUrls: ['./user-edit.component.css']
})
export class UserEditComponent {
  userForm: FormGroup;
  errorMessage: string | null = null;

  constructor(
    public dialogRef: MatDialogRef<UserEditComponent>,
    @Inject(MAT_DIALOG_DATA) public data: User,
    private fb: FormBuilder,
    private userService: UserService
  ) {
    this.userForm = this.fb.group({
      id: [data.id],
      username: [data.username, Validators.required],
      email: [data.email, [Validators.required, Validators.email]]
    });
  }

  onSubmit() {
    if (this.userForm.valid) {
      this.userService.updateUser(this.userForm.value, this.data.id).subscribe({
        next: (updatedUser) => {
          this.dialogRef.close(updatedUser);
        },
        error: (err) => {
          console.log('test 1')
          console.log(err)
          if (err.error && err.error.error === 'username_or_email_exists') {
            this.errorMessage = 'Username or email already exists. Please choose another one.';
          } else {
            this.errorMessage = 'An unexpected error occurred. Please try again later.';
          }
        }
      });
    } else {
      this.userForm.markAllAsTouched();
    }
  }

  onCancel(): void {
    this.dialogRef.close();
  }
}
