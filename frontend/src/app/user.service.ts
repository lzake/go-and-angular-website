import { Injectable } from '@angular/core';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, retry } from 'rxjs/operators';
import { User } from './user';

@Injectable({
  providedIn: 'root'
})
export class UserService {
  private apiUrl = 'http://localhost:8080/users';

  constructor(private http: HttpClient) { }

  getUsers(): Observable<User[]> {
    return this.http.get<User[]>(this.apiUrl)
      .pipe(
        retry(3),
        catchError(this.handleError)
      );
  }

  getUser(ID: number): Observable<User> {
    const url = `${this.apiUrl}/${ID}`;
    return this.http.get<User>(url).pipe(
      retry(3),
      catchError(this.handleError)
    );
  }

  createUser(user: User): Observable<User> {
    return this.http.post<User>(this.apiUrl, user).pipe(
      catchError(error => {
        if (error instanceof HttpErrorResponse && error.status === 400 && error.error && error.error.error === 'username_or_email_exists') {
          return throwError(() => new Error('Username or email already exists. Please choose another one.'));
        }
        return throwError(() => new Error('An unexpected error occurred. Please try again later.'));
      })
    );
  }

  updateUser(user: User, id: number): Observable<User> {
    const url = `${this.apiUrl}/${id}`;
    return this.http.put<User>(url, user).pipe(
      catchError(error => {
        if (error instanceof HttpErrorResponse && error.status === 400 && error.error && error.error.error === 'username_or_email_exists') {
          return throwError(() => new Error('Username or email already exists. Please choose another one.'));
        }
        return throwError(() => new Error('An unexpected error occurred. Please try again later.'));
      })
    );
  }


  deleteUser(id: number): Observable<unknown> {
    const url = `${this.apiUrl}/${id}`;
    return this.http.delete(url).pipe(
      catchError(this.handleError)
    );
  }

  private handleError(error: HttpErrorResponse) {
    if (error.status === 0) {
      console.error('An error occurred:', error.error);
    } else {
      console.error(
        `Backend returned code ${error.status}, body was: `, error.error);
    }
    return throwError(() => new Error('Something bad happened; please try again later.'));
  }
}