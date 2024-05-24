import { Component } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { MatDialogRef } from '@angular/material/dialog'; 
import { User } from '../user';

@Component({
  selector: 'app-user-create',
  templateUrl: './user-create.component.html', 
  styleUrls: ['./user-create.component.css'] 
})
export class UserCreateComponent {
  userForm: FormGroup;

  constructor(
    public dialogRef: MatDialogRef<UserCreateComponent>, 
    private fb: FormBuilder
  ) {
    this.userForm = this.fb.group({
      username: ['', Validators.required], 
      email: ['', [Validators.required, Validators.email]] 
    });
  }

  onSubmit() {
    if (this.userForm.valid) {
      const newUser: User = this.userForm.value; 
      this.dialogRef.close(newUser); 
    } else {
      this.userForm.markAllAsTouched();
    }
  }

  onCancel(): void {
    this.dialogRef.close(); 
  }
}