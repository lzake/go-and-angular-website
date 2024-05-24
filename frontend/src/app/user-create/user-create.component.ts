import { Component } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { MatDialogRef } from '@angular/material/dialog'; 
import { User } from '../user';
import { UserService } from '../user.service'; 

@Component({
  selector: 'app-user-create',
  templateUrl: './user-create.component.html', 
  styleUrls: ['./user-create.component.css'] 
})
export class UserCreateComponent {
  userForm: FormGroup;
  errorMessage: string | null = null; 

  constructor(
    public dialogRef: MatDialogRef<UserCreateComponent>, 
    private fb: FormBuilder,
    private userService: UserService
  ) {
    this.userForm = this.fb.group({
      username: ['', Validators.required], 
      email: ['', [Validators.required, Validators.email]] 
    });
  }

  onSubmit() {
    if (this.userForm.valid) {
      this.userService.createUser(this.userForm.value).subscribe({
        next: (createdUser) => {
          this.dialogRef.close(createdUser); 
        },
        error: (err) => {
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